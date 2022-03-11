# sanity-pcc 7a Bug analysis
This bug was also recorded at: https://jira.whamcloud.com/browse/LU-14346
This one is first occuring on Arm64, but if we do a small change, we can easily reproduced it on X86_64.
Check the session `Why x86_64 can not meet this issue first` for more details

## Hanging analysis:
https://github.com/lustre/lustre-release/blob/master/lustre/tests/multiop.c#L725 as below:
```c
    case 'W':
        for (i = 0; i < mmap_len && mmap_ptr; i += 4096)
            mmap_ptr[i] += junk++;
        break;
```

## Why x86_64 can not meet this issue first
### do_page_fault mechanism varies on aarch64 and x86_64
Let's focus on `mmap_ptr[i] += junk++;`. Traditionally, this process is:
1. read from mmap_ptr[i]first(Execute the read page fault)
2. Write a value to the same page(execute the page_mkwrite to change the page to writable).

But on different platform, it executes quite different.
On aarch64 platform:
- do_page_fault, no FAULT_FLAG_WRITE set, so handle_pte_fault will call do_read_fault
- call do_wp_page --> do_page_mkwrite

On X86_64 platform, the mechanism is different. On X86_64, the no do_read_fault, only do_shared_fault.
- do_page_fault, with FAULT_FLAG_WRITE set, so handle_pte_fault will call do_shared_fault.
- do_shared_fault process
    __do_fault
    do_page_mkwrite
    finish_fault
    fault_dirty_shared_page

do_page_fault, we injection error 0x1412, and then the do_page_fault will return VM_FAULT_RETRY | VM_FAULT_NOPAGE.
We can see the code below:
```c
tmp = do_page_mkwrite(vmf);
if (unlikely(!tmp ||
        (tmp & (VM_FAULT_ERROR | VM_FAULT_NOPAGE)))) {
    put_page(vmf->page);
    return tmp;
}
ret |= finish_fault(vmf);
```
If return VM_FAULT_NOPAGE, we can see that the put_page will be called and return. The finish_fault will be in charge of mapping the page to the page table entry,
and it will be never run. So next time retry, the vmf->pte is none, so the handle_pte_fault will trigger do_shared_fault another time. And then we will fall back to 
Lustre common I/O path.

### Reproduced on X86_64 with a small change
If we change the code like below, we can easily reproduced this bug on `X86_64`. The only difference is we explicitly read from mmap_ptr[i] and then write.
In this case, it will trigger the read page fault, and call do_wp_page. Hanging forever the same with it on Arm64.
```c
case 'W':
    for (i = 0; i < mmap_len && mmap_ptr; i += 4096){
        int test_value = mmap_ptr[i];
        test_value += junk++;
        mmap_ptr[i] += test_value;
    }
    break;
```

## Kernel do_page_fault process analysis
```c
The Page Fault:
do_page_fault(unsigned long addr, unsigned int esr, struct pt_regs *regs)
__do_page_fault(struct mm_struct *mm, unsigned long addr, unsigned int mm_flags, unsigned long vm_flags, struct task_struct *tsk)
mm/memory.c
    handle_mm_fault(struct vm_area_struct *vma, unsigned long address, unsigned int flags)
        __handle_mm_fault(struct vm_area_struct *vma, unsigned long address, unsigned int flags)
            Compose the vmf struct from vma, address and flags.
            handle_pte_fault(struct vm_fault *vmf)


```

The Page Fault core logical is in handle_pte_fault(struct vm_fault *vmf)
```c
    ........
	if (!vmf->pte) {
		if (vma_is_anonymous(vmf->vma))
			return do_anonymous_page(vmf);
		else
			return do_fault(vmf);
	}
    ........
    if (vmf->flags & FAULT_FLAG_WRITE) {
		if (!pte_write(entry))
			return do_wp_page(vmf);
		entry = pte_mkdirty(entry);
	}

In do_fault, according to different flags, run different fault handler. After the do_(*)_fault, the vmf->page will be ready and mapped into page tables.

    else if (!(vmf->flags & FAULT_FLAG_WRITE))
		ret = do_read_fault(vmf);
	else if (!(vma->vm_flags & VM_SHARED))
		ret = do_cow_fault(vmf);
	else
		ret = do_shared_fault(vmf);
```

The do_read_fault and do_shared_fault process is as below:
```c
    do_fault(struct vm_fault *vmf)
        do_read_fault:
            __do_fault
            finish_fault
            unlock_page
            vmf->flags is VM_FAULT_LOCKED
        do_shared_fault:
            __do_fault
            do_page_mkwrite 
            finish_fault
            fault_dirty_shared_page
```
The function __do_fault above will finally call `ll_fault`.
The function do_page_mkwrite above will finally call `ll_page_mkwrite`.

After Page Fault, next time do_page_fault vmf->pte is not NULL, vmf->flags FAULT_FLAG_WRITE, so it will comes to do_wp_page to change the page to writable:
```c
    do_wp_page
        wp_page_shared(vmf):
            get_page
            do_page_mkwrite: if the return value is: (VM_FAULT_ERROR | VM_FAULT_NOPAGE) then put_page and return
            finish_mkwrite_fault(vmf)
```
do_page_mkwrite call ll_page_mkwrite.
    Insert the issue 0x1412 OBD_FAIL_LLITE_PCC_DETACH_MKWRITE.
    Return with VM_FAULT_RETRY | VM_FAULT_NOPAGE
    RETRY again, due to PTE is not NULL, vmf->flags FAULT_FLAG_WRITE, will call do_wp_page again.
So that next time we will enter into do_page_mkwrite again.
We can see how Lustre deal with this injection

### Lustre PCC
ll_page_mkwrite
    pcc_page_mkwrite

```c
	/*
	 * This fault injection can also be used to simulate -ENOSPC and
	 * -EDQUOT failure of underlying PCC backend fs.
	 */
	if (OBD_FAIL_CHECK(OBD_FAIL_LLITE_PCC_DETACH_MKWRITE)) {
		pcc_io_fini(inode);
		pcc_ioctl_detach(inode, PCC_DETACH_OPT_UNCACHE);
		mmap_read_unlock(mm);
		RETURN(VM_FAULT_RETRY | VM_FAULT_NOPAGE);
	}
```
In pcc_page_mkwrite, fail loc 0x1412 and we do pcc_ioctl_detach. Return

Next time Retry, the kernel do_page_fault will enter into pcc_page_mkwrite again. do like below:
```c
	pcc_io_init(inode, PIT_PAGE_MKWRITE, cached);
	if (!*cached) {
		/* This happens when the file is detached from PCC after got
		 * the fault page via ->fault() on the inode of the PCC copy.
		 * Here it can not simply fall back to normal Lustre I/O path.
		 * The reason is that the address space of fault page used by
		 * ->page_mkwrite() is still the one of PCC inode. In the
		 * normal Lustre ->page_mkwrite() I/O path, it will be wrongly
		 * handled as the address space of the fault page is not
		 * consistent with the one of the Lustre inode (though the
		 * fault page was truncated).
		 * As the file is detached from PCC, the fault page must
		 * be released frist, and retry the mmap write (->fault() and
		 * ->page_mkwrite).
		 * We use an ugly and tricky method by returning
		 * VM_FAULT_NOPAGE | VM_FAULT_RETRY to the caller
		 * __do_page_fault and retry the memory fault handling.
		 */
		if (page->mapping == pcc_file->f_mapping) {
			*cached = true;
			mmap_read_unlock(mm);
			RETURN(VM_FAULT_RETRY | VM_FAULT_NOPAGE);
		}
```
Next time retry, the kernel do_page_fault will retry, forever, and the process is hang.

## Solution
As the above code snappit shows, we want to let kernel to retry retry the mmap write (->fault() and ->page_mkwrite).
In handle_pte_fault, we know that we must make sure that the page is not mapped and removed from pagecache, so that the
__do_page_fault will try the memory fault handling. 

The easy fix here is to remove the page and page table entry when we do fail injection in pcc_page_mkwrite. But I don't 
find a good method to execute this, so list the info here and ask for community help.

Some tried fix is:
add function: generic_error_remove_page, but the mapped page still can not unmapped, so it is failed to remove from page cache.
error like:
```c
[ 1028.598422] BUG: Bad page cache in process lt-multiop  pfn:27d2f
[ 1028.599152] page:ffff7fecf13f4bc0 refcount:3 mapcount:1 mapping:0000000062843dba index:0x0
[ 1028.600160] ext4_da_aops [ext4] name:"0x200000402:0x3:0x0"
[ 1028.600816] flags: 0xfffff800000006d(locked|referenced|uptodate|lru|active)
[ 1028.601635] raw: 0fffff800000006d ffff7fecf13f5688 ffff7fecf13fea08 ffffb3c3a49ebe00
[ 1028.602608] raw: 0000000000000000 0000000000000000 0000000300000000 ffffb3c385340000
[ 1028.603563] page dumped because: still mapped when deleted
[ 1028.604249] pages's memcg:ffffb3c385340000
[ 1028.604768] CPU: 4 PID: 8087 Comm: lt-multiop Kdump: loaded Tainted: G        W  OE    --------- -  - 4.18.0-348.2.1.el8_lustre_debugdebug.aarch64 #1
[ 1028.606401] Hardware name: QEMU KVM Virtual Machine, BIOS 0.0.0 02/06/2015
[ 1028.607254] Call trace:
[ 1028.607586]  dump_backtrace+0x0/0x310
[ 1028.608050]  show_stack+0x28/0x38
[ 1028.608489]  dump_stack+0xf0/0x12c
[ 1028.608948]  unaccount_page_cache_page+0x320/0x718
[ 1028.609577]  __delete_from_page_cache+0x180/0x900
[ 1028.610204]  delete_from_page_cache+0x8c/0xe0
[ 1028.610780]  generic_error_remove_page+0xa8/0xf0
[ 1028.611432]  pcc_page_mkwrite+0xc04/0x1df0 [lustre]
[ 1028.612108]  ll_page_mkwrite+0x480/0x1af0 [lustre]
[ 1028.612740]  do_page_mkwrite+0x168/0x300
[ 1028.613265]  do_wp_page+0xe4c/0x18c8
[ 1028.613751]  __handle_mm_fault+0xb74/0x10f0
[ 1028.614307]  handle_mm_fault+0x428/0x710
[ 1028.614831]  do_page_fault+0x3a0/0xb38
[ 1028.615332]  do_mem_abort+0x74/0x188
[ 1028.615807]  el0_da+0x20/0x24
```
