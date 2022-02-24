# Lustre PCC
Persistent Client Cache 代码核心注释

PCC与Kernel交互
```c
The first Page Fault:
arch/arm64/mm/fault.c
do_page_fault(unsigned long addr, unsigned int esr, struct pt_regs *regs)
__do_page_fault(struct mm_struct *mm, unsigned long addr, unsigned int mm_flags, unsigned long vm_flags, struct task_struct *tsk)
mm/memory.c
    handle_mm_fault(struct vm_area_struct *vma, unsigned long address, unsigned int flags)
        __handle_mm_fault(struct vm_area_struct *vma, unsigned long address, unsigned int flags)
            Compose the vmf struct from vma, address and flags.
            handle_pte_fault(struct vm_fault *vmf)
                do_fault(struct vm_fault *vmf)
                    do_read_fault( the pcc_fault is called here in arm64)
                    do_cow_fault
                    do_shared_fault( the pcc_fault is called here in X86_64)
```                    
The totally page_fault chain on Arm64 is:

```c
do_page_fault triagger with:
    unsigned long vm_flags = VM_READ | VM_WRITE;
	unsigned int mm_flags = FAULT_FLAG_ALLOW_RETRY | FAULT_FLAG_KILLABLE;
	if (is_el0_instruction_abort(esr)) {
		vm_flags = VM_EXEC;
	} else if ((esr & ESR_ELx_WNR) && !(esr & ESR_ELx_CM)) {
		vm_flags = VM_WRITE;
		mm_flags |= FAULT_FLAG_WRITE;
	}
```
The first time enter into here, the esr & ESR_ELx_WNR is 0, so it is a `read_fault`.
```c
in __do_page_fault, compose the struct vm_area_struct *vma.
    struct vm_area_struct *vma;
	vma = find_vma(mm, addr);

in __handle_mm_fault, compose the struct vm_fault_t vmf. The flags is the mm_flags.
    struct vm_fault vmf = {
		.vma = vma,
		.address = address & PAGE_MASK,
		.flags = flags,
		.pgoff = linear_page_index(vma, address),
		.gfp_mask = __get_fault_gfp_mask(vma),
	};

in handle_pte_fault, according to different flags, run different branch:
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

in do_fault, according to different flags, run different fault.
    else if (!(vmf->flags & FAULT_FLAG_WRITE))
		ret = do_read_fault(vmf);
	else if (!(vma->vm_flags & VM_SHARED))
		ret = do_cow_fault(vmf);
	else
		ret = do_shared_fault(vmf);
    
When run mmap(Read and Write) to the pcc, the first time execute the write, in arm64 it is 
only triagger the do_read_fault. But on X86_64, it triagger do_shared_fault.

do_read_fault:
    __do_fault
    finish_fault
    unlock_page
    vmf->flags is VM_FAULT_LOCKED
    
do_shared_fault:
    __do_fault
    do_page_mkwrite -> Insert the PCC 0x1412 issue.
    finish_fault
    fault_dirty_shared_page

After return from the first time, the next time of the do_page_fault will enter into handle_pte_fault:
According to the !vmf->pte, do_fault or do_wp_page.

in Aarch64:
    do_wp_page
        do_page_mkwrite: The PTE here is not NULL, and vmf->flags FAULT_FLAG_WRITE is set.
        Insert the issue 0x1412 OBD_FAIL_LLITE_PCC_DETACH_MKWRITE.
        Return with VM_FAULT_RETRY | VM_FAULT_NOPAGE
        RETRY again, due to PTE is not NULL, vmf->flags FAULT_FLAG_WRITE, do_wp_page again.
        do_wp_page
            vmf->page = vm_normal_page(vma, vmf->address, vmf->orig_pte);
            ........
            else if (unlikely((vma->vm_flags & (VM_WRITE|VM_SHARED)) ==
                (VM_WRITE|VM_SHARED))) {
                return wp_page_shared(vmf);
            }
            wp_page_shared:
                get_page(vmf->page);
                pte_unmap_unlock(vmf->pte, vmf->ptl);
                tmp = do_page_mkwrite(vmf);
                if (unlikely(!tmp || (tmp &
                              (VM_FAULT_ERROR | VM_FAULT_NOPAGE)))) {
                    put_page(vmf->page);
                    return tmp;
                }//很不幸触发了此处，返回，下方不运行
                tmp = finish_mkwrite_fault(vmf);
                if (unlikely(tmp & (VM_FAULT_ERROR | VM_FAULT_NOPAGE))) {
                    unlock_page(vmf->page);
                    put_page(vmf->page);
                    return tmp;
                }
        do_page_mkwrite: pcc_io_init find it is not cached. 
        if (!*cached) {
            if (page->mapping == pcc_file->f_mapping) {
                *cached = true;
			    mmap_read_unlock(mm);
			    RETURN(VM_FAULT_RETRY | VM_FAULT_NOPAGE);
            }
        }
        ll_page_mkwrite:
            if (cached)
                goto out;
in X86_64:
    pcc_fault for another time, The PTE is NULL.
    do_page_mkwrite.

```

每次进入page_fault，vma是在__do_page_fault中，根据addr，mm找到的：vma = find_vma(mm, addr);
而每次进入page_fault, vmf都会在__handle_mm_fault中被重组，因此认为修改vmf，没有作用。

```C
// Lustre的vm_ops注册的fault函数，对应kernel的do_fault代码
static vm_fault_t ll_fault(struct vm_fault *vmf)
{
	struct vm_area_struct *vma = vmf->vma;
	int count = 0;
	bool printed = false;
	bool cached;
	vm_fault_t result;
	ktime_t kstart = ktime_get();
	sigset_t old, new;

    //这里调用pcc_fault，检查page_fault是否已经存在与pcc中
	result = pcc_fault(vma, vmf, &cached);
    // 返回cached，则跳转到out，并返回
	if (cached)
		goto out;

	CDEBUG(D_MMAP|D_IOTRACE,
	       DFID": vma=%p start=%#lx end=%#lx vm_flags=%#lx idx=%lu\n",
	       PFID(&ll_i2info(file_inode(vma->vm_file))->lli_fid),
	       vma, vma->vm_start, vma->vm_end, vma->vm_flags, vmf->pgoff);

	/* Only SIGKILL and SIGTERM is allowed for fault/nopage/mkwrite
	 * so that it can be killed by admin but not cause segfault by
	 * other signals.
	 */
	siginitsetinv(&new, sigmask(SIGKILL) | sigmask(SIGTERM));
	sigprocmask(SIG_BLOCK, &new, &old);

	/* make sure offset is not a negative number */
	if (vmf->pgoff > (MAX_LFS_FILESIZE >> PAGE_SHIFT))
		return VM_FAULT_SIGBUS;

restart:
	result = ll_fault0(vma, vmf);
	if (vmf->page &&
	    !(result & (VM_FAULT_RETRY | VM_FAULT_ERROR | VM_FAULT_LOCKED))) {
		struct page *vmpage = vmf->page;

		/* check if this page has been truncated */
		lock_page(vmpage);
		if (unlikely(vmpage->mapping == NULL)) { /* unlucky */
			unlock_page(vmpage);
			put_page(vmpage);
			vmf->page = NULL;

			if (!printed && ++count > 16) {
				CWARN("the page is under heavy contention, maybe your app(%s) needs revising :-)\n",
				      current->comm);
				printed = true;
			}

			goto restart;
		}

		result |= VM_FAULT_LOCKED;
	}
	sigprocmask(SIG_SETMASK, &old, NULL);

out:
	if (vmf->page && result == VM_FAULT_LOCKED) {
		ll_rw_stats_tally(ll_i2sbi(file_inode(vma->vm_file)),
				  current->pid, vma->vm_file->private_data,
				  cl_offset(NULL, vmf->page->index), PAGE_SIZE,
				  READ);
		ll_stats_ops_tally(ll_i2sbi(file_inode(vma->vm_file)),
				   LPROC_LL_FAULT,
				   ktime_us_delta(ktime_get(), kstart));
	}

	return result;
}
```

```C
int pcc_fault(struct vm_area_struct *vma, struct vm_fault *vmf,
	      bool *cached)
{
	struct file *file = vma->vm_file;
	struct inode *inode = file_inode(file);
	struct ll_file_data *fd = file->private_data;

    // pccf_file: Opened cache file of pcc_file of ll_file_data
	struct file *pcc_file = fd->fd_pcc_file.pccf_file;
	struct vm_operations_struct *pcc_vm_ops = vma->vm_private_data;
	int rc;

	ENTRY;

    // 判断pcc文件是否存在，pcc的vm_ops方法集合和是否实现了fault操作
	if (!pcc_file || !pcc_vm_ops || !pcc_vm_ops->fault) {
		*cached = false;
		RETURN(0);
	}

    // io初始化
	pcc_io_init(inode, PIT_FAULT, cached);
    // 没找到cache，返回
	if (!*cached)
		RETURN(0);

	vma->vm_file = pcc_file;
    // 调用vma_ops的实现方法在backend文件系统中进行do_fault操作
#ifdef HAVE_VM_OPS_USE_VM_FAULT_ONLY
	rc = pcc_vm_ops->fault(vmf);
#else
	rc = pcc_vm_ops->fault(vma, vmf);
#endif
	vma->vm_file = file;

	pcc_io_fini(inode);
	RETURN(rc);
}

static void __pcc_layout_invalidate(struct pcc_inode *pcci)
{
	pcci->pcci_type = LU_PCC_NONE;
	pcc_layout_gen_set(pcci, CL_LAYOUT_GEN_NONE);
	if (atomic_read(&pcci->pcci_active_ios) == 0)
		return;

	CDEBUG(D_CACHE, "Waiting for IO completion: %d\n",
		       atomic_read(&pcci->pcci_active_ios));
	wait_event_idle(pcci->pcci_waitq,
			atomic_read(&pcci->pcci_active_ios) == 0);
}

```

pcc_io_init函数
```C
// 参数
// inode: 进程地址空间映射文件的inode对象
// iot: pcc io type
// cached: 是否在pcc中被缓存
static void pcc_io_init(struct inode *inode, enum pcc_io_type iot, bool *cached)
{
	struct pcc_inode *pcci;

    // inode操作加锁
	pcc_inode_lock(inode);
    // 根据inode找到对应的struct ll_inode_info
	pcci = ll_i2pcci(inode);
    // 检查pcci是否存在，如果存在则检查其是pcci_layout_gen的field值
	if (pcci && pcc_inode_has_layout(pcci)) {
		LASSERT(atomic_read(&pcci->pcci_refcount) > 0);
		atomic_inc(&pcci->pcci_active_ios);
		*cached = true;
	} else {
        // 没有找到cache数据，则尝试建立
		*cached = false;
		if (pcc_may_auto_attach(inode, iot)) {
			(void) pcc_try_auto_attach(inode, cached, iot);
			if (*cached) {
				pcci = ll_i2pcci(inode);
                // If reference count is 0, then the cache is not inited, if 1, then no one is using it.
				LASSERT(atomic_read(&pcci->pcci_refcount) > 0);
                // pcci->pcci_active_ios: How many IOs are on going on this cached object.
                // Layout can be changed only if there is no active IO.
                // 该函数对原子类型变量v原子地增加1
				atomic_inc(&pcci->pcci_active_ios);
			}
		}
	}
	pcc_inode_unlock(inode);
}

static inline struct ll_inode_info *ll_i2info(struct inode *inode)
{
    // container_of()的作用就是通过一个结构变量中一个成员的地址找到这个结构体变量的首地址。
    // Ref: https://blog.csdn.net/s2603898260/article/details/79371024
    // container_of(ptr, type, member)已知结构体type的成员member的地址ptr，求解结构体type的起始地址。
    return container_of(inode, struct ll_inode_info, lli_vfs_inode);
}

// 返回ll_inode_info对象的lli_pcc_inode
static inline struct pcc_inode *ll_i2pcci(struct inode *inode)
{
    return ll_i2info(inode)->lli_pcc_inode;
}

static inline bool pcc_inode_has_layout(struct pcc_inode *pcci)
{
    return pcci->pcci_layout_gen != CL_LAYOUT_GEN_NONE;
}

static void pcc_io_fini(struct inode *inode)
{
    struct pcc_inode *pcci = ll_i2pcci(inode);
    // 判断pcci_active_ios是否大于0
    LASSERT(pcci && atomic_read(&pcci->pcci_active_ios) > 0);
    // atomic_dec_and_test()返回一个整型值，如果原子类型v在减1后变为0，则返回1，否则返回0。
    if (atomic_dec_and_test(&pcci->pcci_active_ios)){
        // wake_up: 只唤醒等待队列中第一个等待的线程．
        // Ref: https://blog.csdn.net/wh0604111092/article/details/78753400
        wake_up(&pcci->pcci_waitq);
    }
}
```

pcc 底层的初始化操作和attach操作，在pcc_try_auto_attach 和pcc_may_auto_attach
```C
/*
 * TODO: For RW-PCC, it is desirable to store HSM info as a layout (LU-10606).
 * Thus the client can get archive ID from the layout directly. When try to
 * attach the file automatically which is in HSM released state (according to
 * LOV_PATTERN_F_RELEASED in the layout), it can determine whether the file is
 * valid cached on PCC more precisely according to the @rwid (archive ID) in
 * the PCC dataset and the archive ID in HSM attrs.
 */
static int pcc_try_auto_attach(struct inode *inode, bool *cached,
			       enum pcc_io_type iot)
{
	struct pcc_super *super = &ll_i2sbi(inode)->ll_pcc_super;
	struct cl_layout clt = {
		.cl_layout_gen = 0,
		.cl_is_released = false,
	};
	struct ll_inode_info *lli = ll_i2info(inode);
	__u32 gen;
	int rc;

	ENTRY;

	/*
	 * Quick check whether there is PCC device.
	 */
	if (list_empty(&super->pccs_datasets))
		RETURN(0);

	/*
	 * The file layout lock was cancelled. And this open does not
	 * obtain valid layout lock from MDT (i.e. the file is being
	 * HSM restoring).
	 */
	if (iot == PIT_OPEN) {
		if (ll_layout_version_get(lli) == CL_LAYOUT_GEN_NONE)
			RETURN(0);
	} else {
		rc = ll_layout_refresh(inode, &gen);
		if (rc)
			RETURN(rc);
	}

	rc = pcc_get_layout_info(inode, &clt);
	if (rc)
		RETURN(rc);

	if (iot != PIT_OPEN && gen != clt.cl_layout_gen) {
		CDEBUG(D_CACHE, DFID" layout changed from %d to %d.\n",
		       PFID(ll_inode2fid(inode)), gen, clt.cl_layout_gen);
		RETURN(-EINVAL);
	}

	if (clt.cl_is_released)
		rc = pcc_try_datasets_attach(inode, iot, clt.cl_layout_gen,
					     LU_PCC_READWRITE, cached);

	RETURN(rc);
}

static inline bool pcc_may_auto_attach(struct inode *inode,
				       enum pcc_io_type iot)
{
    // 根据lustre inode找到lustre_inode_info
	struct ll_inode_info *lli = ll_i2info(inode);
	struct pcc_super *super = ll_i2pccs(inode);

	/* Known the file was not in any PCC backend. */
    // pcc dataset flag:pcc_dataset_flags, 所有flag定义在llite/pcc.h中
	if (lli->lli_pcc_dsflags & PCC_DATASET_NONE)
		return false;

    // lli_pcc_generation saves the gobal PCC generation
    // when the file was successfully attached into PCC.
	/*
	 * lli_pcc_generation == 0 means that the file was never attached into
	 * PCC, or may be once attached into PCC but detached as the inode is
	 * evicted from icache (i.e. "echo 3 > /proc/sys/vm/drop_caches" or
	 * icache shrinking due to the memory pressure), which will cause the
	 * file detach from PCC when releasing the inode from icache.
	 * In either case, we still try to attach.
	 */
	/* lli_pcc_generation == 0, or the PCC setting was changed,
	 * or there is no PCC setup on the client and the try will return
	 * immediately in pcc_try_auto_attach().
	 */
	if (super->pccs_generation != lli->lli_pcc_generation)
		return true;

	/* The cached setting @lli_pcc_dsflags is valid */
	if (iot == PIT_OPEN)
		return lli->lli_pcc_dsflags & PCC_DATASET_OPEN_ATTACH;

	if (iot == PIT_GETATTR)
		return lli->lli_pcc_dsflags & PCC_DATASET_STAT_ATTACH;

	return lli->lli_pcc_dsflags & PCC_DATASET_IO_ATTACH;
}
```

```asm
static int pcc_try_datasets_attach(struct inode *inode, enum pcc_io_type iot,
				   __u32 gen, enum lu_pcc_type type,
				   bool *cached)
{
	struct pcc_super *super = &ll_i2sbi(inode)->ll_pcc_super;
	struct ll_inode_info *lli = ll_i2info(inode);
	struct pcc_dataset *dataset = NULL, *tmp;
	int rc = 0;

	ENTRY;

	down_read(&super->pccs_rw_sem);
	list_for_each_entry_safe(dataset, tmp,
				 &super->pccs_datasets, pccd_linkage) {
		if (!pcc_auto_attach_enabled(dataset->pccd_flags, iot))
			break;

		rc = pcc_try_dataset_attach(inode, gen, type, dataset, cached);
		if (rc < 0 || (!rc && *cached))
			break;
	}

	/*
	 * Update the saved dataset flags for the inode accordingly if failed.
	 */
	if (!rc && !*cached) {
		/*
		 * Currently auto attach strategy for a PCC backend is
		 * unchangeable once once it was added into the PCC datasets on
		 * a client as the support to change auto attach strategy is
		 * not implemented yet.
		 */
		/*
		 * If tried to attach from one PCC backend:
		 * @lli_pcc_generation > 0:
		 * 1) The file was once attached into PCC, but now the
		 * corresponding PCC backend should be removed from the client;
		 * 2) The layout generation was changed, the data has been
		 * restored;
		 * 3) The corresponding PCC copy is not existed on PCC
		 * @lli_pcc_generation == 0:
		 * The file is never attached into PCC but in a HSM released
		 * state, or once attached into PCC but the inode was evicted
		 * from icache later.
		 * Set the saved dataset flags with PCC_DATASET_NONE. Then this
		 * file will skip from the candidates to try auto attach until
		 * the file is attached into PCC again.
		 *
		 * If the file was never attached into PCC, or once attached but
		 * its inode was evicted from icache (lli_pcc_generation == 0),
		 * or the corresponding dataset was removed from the client,
		 * set the saved dataset flags with PCC_DATASET_NONE.
		 *
		 * TODO: If the file was once attached into PCC but not try to
		 * auto attach due to the change of the configuration parameters
		 * for this dataset (i.e. change from auto attach enabled to
		 * auto attach disabled for this dataset), update the saved
		 * dataset flags with the found one.
		 */
		lli->lli_pcc_dsflags = PCC_DATASET_NONE;
	}
	up_read(&super->pccs_rw_sem);

	RETURN(rc);
}

static int pcc_try_dataset_attach(struct inode *inode, __u32 gen,
				  enum lu_pcc_type type,
				  struct pcc_dataset *dataset,
				  bool *cached)
{
	struct ll_inode_info *lli = ll_i2info(inode);
	struct pcc_inode *pcci = lli->lli_pcc_inode;
	const struct cred *old_cred;
	struct dentry *pcc_dentry = NULL;
	char pathname[PCC_DATASET_MAX_PATH];
	__u32 pcc_gen;
	int rc;

	ENTRY;

	if (type == LU_PCC_READWRITE &&
	    !(dataset->pccd_flags & PCC_DATASET_RWPCC))
		RETURN(0);

	rc = pcc_fid2dataset_path(pathname, PCC_DATASET_MAX_PATH,
				  &lli->lli_fid);

	old_cred = override_creds(pcc_super_cred(inode->i_sb));
	pcc_dentry = pcc_lookup(dataset->pccd_path.dentry, pathname);
	if (IS_ERR(pcc_dentry)) {
		rc = PTR_ERR(pcc_dentry);
		CDEBUG(D_CACHE, "%s: path lookup error on "DFID":%s: rc = %d\n",
		       ll_i2sbi(inode)->ll_fsname, PFID(&lli->lli_fid),
		       pathname, rc);
		/* ignore this error */
		GOTO(out, rc = 0);
	}

	rc = ll_vfs_getxattr(pcc_dentry, pcc_dentry->d_inode, pcc_xattr_layout,
			     &pcc_gen, sizeof(pcc_gen));
	if (rc < 0)
		/* ignore this error */
		GOTO(out_put_pcc_dentry, rc = 0);

	rc = 0;
	/* The file is still valid cached in PCC, attach it immediately. */
	if (pcc_gen == gen) {
		CDEBUG(D_CACHE, DFID" L.Gen (%d) consistent, auto attached.\n",
		       PFID(&lli->lli_fid), gen);
		if (!pcci) {
			OBD_SLAB_ALLOC_PTR_GFP(pcci, pcc_inode_slab, GFP_NOFS);
			if (pcci == NULL)
				GOTO(out_put_pcc_dentry, rc = -ENOMEM);

			pcc_inode_init(pcci, lli);
			dget(pcc_dentry);
			pcc_inode_attach_init(dataset, pcci, pcc_dentry, type);
		} else {
			/*
			 * This happened when a file was once attached into
			 * PCC, and some processes keep this file opened
			 * (pcci->refcount > 1) and corresponding PCC file
			 * without any I/O activity, and then this file was
			 * detached by the manual detach command or the
			 * revocation of the layout lock (i.e. cached LRU lock
			 * shrinking).
			 */
			pcc_inode_get(pcci);
			pcci->pcci_type = type;
		}
		pcc_inode_dsflags_set(lli, dataset);
		pcc_layout_gen_set(pcci, gen);
		*cached = true;
	}
out_put_pcc_dentry:
	dput(pcc_dentry);
out:
	revert_creds(old_cred);
	RETURN(rc);
}

```

主要需要判断，vmf在Lustre中如何进行流转，其pte对象被如何改变。
ll_fault -> pcc_fault -> cached, ll_rw_stats_tally, ll_stats_ops_tally, then return
ll_page_mkwrite -> pcc_page_mkwrite -> out return

```asm
rc = pcc_vm_ops->fault(vmf);

底层调用 fs/ext4/inode.c
vm_fault_t ext4_filemap_fault(struct vm_fault *vmf)
{
	struct inode *inode = file_inode(vmf->vma->vm_file);
	vm_fault_t ret;

	down_read(&EXT4_I(inode)->i_mmap_sem);
	ret = filemap_fault(vmf);
	up_read(&EXT4_I(inode)->i_mmap_sem);

	return ret;
}

filemap_fault 为mm/filemap.c:

/**
 * filemap_fault - read in file data for page fault handling
 * @vmf:	struct vm_fault containing details of the fault
 *
 * filemap_fault() is invoked via the vma operations vector for a
 * mapped memory region to read in file data during a page fault.
 *
 * The goto's are kind of ugly, but this streamlines the normal case of having
 * it in the page cache, and handles the special cases reasonably without
 * having a lot of duplicated code.
 *
 * vma->vm_mm->mmap_sem must be held on entry.
 *
 * If our return value has VM_FAULT_RETRY set, it's because
 * lock_page_or_retry() returned 0.
 * The mmap_sem has usually been released in this case.
 * See __lock_page_or_retry() for the exception.
 *
 * If our return value does not have VM_FAULT_RETRY set, the mmap_sem
 * has not been released.
 *
 * We never return with VM_FAULT_RETRY and a bit from VM_FAULT_ERROR set.
 */
vm_fault_t filemap_fault(struct vm_fault *vmf)
{
	int error;
	struct file *file = vmf->vma->vm_file;
	struct address_space *mapping = file->f_mapping;
	struct file_ra_state *ra = &file->f_ra;
	struct inode *inode = mapping->host;
	pgoff_t offset = vmf->pgoff;
	pgoff_t max_off;
	struct page *page;
	vm_fault_t ret = 0;

	max_off = DIV_ROUND_UP(i_size_read(inode), PAGE_SIZE);
	if (unlikely(offset >= max_off))
		return VM_FAULT_SIGBUS;

	/*
	 * Do we have something in the page cache already?
	 */
	page = find_get_page(mapping, offset);
	if (likely(page) && !(vmf->flags & FAULT_FLAG_TRIED)) {
		/*
		 * We found the page, so try async readahead before
		 * waiting for the lock.
		 */
		do_async_mmap_readahead(vmf->vma, ra, file, page, offset);
	} else if (!page) {
		/* No page in the page cache at all */
		do_sync_mmap_readahead(vmf->vma, ra, file, offset);
		count_vm_event(PGMAJFAULT);
		count_memcg_event_mm(vmf->vma->vm_mm, PGMAJFAULT);
		ret = VM_FAULT_MAJOR;
retry_find:
		page = find_get_page(mapping, offset);
		if (!page)
			goto no_cached_page;
	}

	if (!lock_page_or_retry(page, vmf->vma->vm_mm, vmf->flags)) {
		put_page(page);
		return ret | VM_FAULT_RETRY;
	}

	/* Did it get truncated? */
	if (unlikely(page->mapping != mapping)) {
		unlock_page(page);
		put_page(page);
		goto retry_find;
	}
	VM_BUG_ON_PAGE(page->index != offset, page);

	/*
	 * We have a locked page in the page cache, now we need to check
	 * that it's up-to-date. If not, it is going to be due to an error.
	 */
	if (unlikely(!PageUptodate(page)))
		goto page_not_uptodate;

	/*
	 * Found the page and have a reference on it.
	 * We must recheck i_size under page lock.
	 */
	max_off = DIV_ROUND_UP(i_size_read(inode), PAGE_SIZE);
	if (unlikely(offset >= max_off)) {
		unlock_page(page);
		put_page(page);
		return VM_FAULT_SIGBUS;
	}

	vmf->page = page;
	return ret | VM_FAULT_LOCKED;

no_cached_page:
	/*
	 * We're only likely to ever get here if MADV_RANDOM is in
	 * effect.
	 */
	error = page_cache_read(file, offset, vmf->gfp_mask);

	/*
	 * The page we want has now been added to the page cache.
	 * In the unlikely event that someone removed it in the
	 * meantime, we'll just come back here and read it again.
	 */
	if (error >= 0)
		goto retry_find;

	/*
	 * An error return from page_cache_read can result if the
	 * system is low on memory, or a problem occurs while trying
	 * to schedule I/O.
	 */
	return vmf_error(error);

page_not_uptodate:
	/*
	 * Umm, take care of errors if the page isn't up-to-date.
	 * Try to re-read it _once_. We do this synchronously,
	 * because there really aren't any performance issues here
	 * and we need to check for errors.
	 */
	ClearPageError(page);
	error = mapping->a_ops->readpage(file, page);
	if (!error) {
		wait_on_page_locked(page);
		if (!PageUptodate(page))
			error = -EIO;
	}
	put_page(page);

	if (!error || error == AOP_TRUNCATED_PAGE)
		goto retry_find;

	/* Things didn't work out. Return zero to tell the mm layer so. */
	shrink_readahead_size_eio(file, ra);
	return VM_FAULT_SIGBUS;
}
EXPORT_SYMBOL(filemap_fault);
```






