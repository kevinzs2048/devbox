# do_page_fault 代码注释

基于v5.0内核, aarch64, arch/arm64/mm/fault.c

## 代码调用流程
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

## 代码详细
do_page_fault 缺页异常修复的核心函数 arch/arm64/mm/fault.c

```C
/*
 * 参数:
 * addr: 异常发生时的虚拟地址，有FSR寄存器提供
 * esr:  异常发生时的异常状态，ESR提供
 * regs: 异常发生时的pt_regs
 */
 
static int __kprobes do_page_fault(unsigned long addr, unsigned int esr,
				   struct pt_regs *regs)
{
	const struct fault_info *inf;
	struct task_struct *tsk;
	struct mm_struct *mm;
	vm_fault_t fault, major = 0;
	unsigned long vm_flags = VM_READ | VM_WRITE;
	unsigned int mm_flags = FAULT_FLAG_ALLOW_RETRY | FAULT_FLAG_KILLABLE;

	if (notify_page_fault(regs, esr))
		return 0;

	tsk = current;
	mm  = tsk->mm;

	/*
	 * If we're in an interrupt or have no user context, we must not take
	 * the fault.
	 */
	/*
	 * faulthandler_disabled：检查异常发生时，内核是否正在执行一些关键路径代码:
	 * 
	 * #define faulthandler_disabled() (pagefault_disabled() || in_atomic()) 
	 * -> 定义在include/linux/uaccess.h中，代码中有详解注释
	 * pagefault_disabled：关闭缺页异常处理
	 * in_atomic:
	 *   - 内核正在执行中断处理
	 *   - 内核在禁用内核抢占情况下执行临界区代码
	 *
	 * !mm的含义：
	 * 内核线程的进程描述符的mm字段总为NULL，如果缺页异常发生在中断上下文或者内核线程中，则直接跳转到no_context，执行
	 * _do_kernel_fault()
	 */
	 
	if (faulthandler_disabled() || !mm)
		goto no_context;

	/*
	 * user_mode：通过PSTATE寄存器判断异常是否发生在EL0. 用户模式发生异常则设置FAULT_FLAG_USER
	 * 定义在arch/arm64/include/asm/ptrace.h
	 * #define user_mode(regs)	\
	 * (((regs)->pstate & PSR_MODE_MASK) == PSR_MODE_EL0t)
	 * cpsr为当前程序状态寄存器，如果M[0:4]均为0，则处于用户模式
     */
	if (user_mode(regs))
		mm_flags |= FAULT_FLAG_USER;

	/*
	 * is_el0_instruction_abort 读取ESR的EC，判断是否为低异常等级的指令异常ESR_ELx_EC_IABT_LOW。
	 * 若异常是EL0中的指令异常，说明这个进程地址空间具有可执行权限
	 */
	if (is_el0_instruction_abort(esr)) {
		vm_flags = VM_EXEC;
	} else if ((esr & ESR_ELx_WNR) && !(esr & ESR_ELx_CM)) {
        /*
         * ESR_ELx_WNR->ISS表中的WnR，值为1则说明写内存区域发生错误；值为0说明读内存区域发生错误
         * 若异常是EL0中的指令异常，说明这个进程地址空间具有可执行权限
         * ESR_ELx_CM 显示 内容待确认
         */
		vm_flags = VM_WRITE;
		mm_flags |= FAULT_FLAG_WRITE;
	}

	/*
	 * is_ttbr0_addr：异常地址发生在用户空间。小于TASK_SIZE的地址是用户空间的地址。TASK_SIZE定义见:
	 * arch/arm64/include/asm/processor.h
	 * is_el1_permission_fault：判断访问权限是否发生在EL1中
	 */
	if (is_ttbr0_addr(addr) && is_el1_permission_fault(addr, esr, regs)) {
		/* regs->orig_addr_limit may be 0 if we entered from EL0 */
		if (regs->orig_addr_limit == KERNEL_DS)
			die_kernel_fault("access to user memory with fs=KERNEL_DS",
					 addr, esr, regs);

		if (is_el1_instruction_abort(esr))
			die_kernel_fault("execution of user memory",
					 addr, esr, regs);

		if (!search_exception_tables(regs->pc))
			die_kernel_fault("access to user memory outside uaccess routines",
					 addr, esr, regs);
	}

	perf_sw_event(PERF_COUNT_SW_PAGE_FAULTS, 1, regs, addr);

	/*
	 * As per x86, we may deadlock here. However, since the kernel only
	 * validly references user space from well defined areas of the code,
	 * we can bug out early if this is from code which shouldn't.
	 */

	/*
	 * 缺页异常没有发生在中断上下文，内核线程以及一些特殊情况。下面检查进程地址空间引发的缺页异常
	 * 首先获取mm->mmap_sem, 即进程的内存描述符中的读写信号量
	 * 返回1，成功获得锁；0的话，锁被别人占用
	 * 获取不到的情况：
	 * 发生在内核空间：内核一般不会随意访问用户空间，只有少数几个函数例如copy_to_user等。为没处
	 *              设置了错误地址表。如果没查到，则调准到_do_kernel_fault
	                 
	 * 发生在用户空间：可以调用down_read来睡眠，等待锁持有者释放
	 */	 
	if (!down_read_trylock(&mm->mmap_sem)) {
		if (!user_mode(regs) && !search_exception_tables(regs->pc))
			goto no_context;
retry:
		down_read(&mm->mmap_sem);
	} else {
		/*
		 * The above down_read_trylock() might have succeeded in which
		 * case, we'll have missed the might_sleep() from down_read().
		 */
		might_sleep();
#ifdef CONFIG_DEBUG_VM
		if (!user_mode(regs) && !search_exception_tables(regs->pc))
			goto no_context;
#endif
	}

	fault = __do_page_fault(mm, addr, mm_flags, vm_flags, tsk);
	major |= fault & VM_FAULT_MAJOR;

    //处理重试情况
	if (fault & VM_FAULT_RETRY) {
		/*
		 * If we need to retry but a fatal signal is pending,
		 * handle the signal first. We do not need to release
		 * the mmap_sem because it would already be released
		 * in __lock_page_or_retry in mm/filemap.c.
		 */
		if (fatal_signal_pending(current)) {
			if (!user_mode(regs))
				goto no_context;
			return 0;
		}

		/*
		 * Clear FAULT_FLAG_ALLOW_RETRY to avoid any risk of
		 * starvation.
		 */
		if (mm_flags & FAULT_FLAG_ALLOW_RETRY) {
			mm_flags &= ~FAULT_FLAG_ALLOW_RETRY;
			mm_flags |= FAULT_FLAG_TRIED;
			goto retry;
		}
	}
	up_read(&mm->mmap_sem);

	/*
	 * Handle the "normal" (no error) case first.
	 */
	if (likely(!(fault & (VM_FAULT_ERROR | VM_FAULT_BADMAP |
			      VM_FAULT_BADACCESS)))) {
		/*
		 * Major/minor page fault accounting is only done
		 * once. If we go through a retry, it is extremely
		 * likely that the page will be found in page cache at
		 * that point.
		 */
		if (major) {
			tsk->maj_flt++;
			perf_sw_event(PERF_COUNT_SW_PAGE_FAULTS_MAJ, 1, regs,
				      addr);
		} else {
			tsk->min_flt++;
			perf_sw_event(PERF_COUNT_SW_PAGE_FAULTS_MIN, 1, regs,
				      addr);
		}

		return 0;
	}

	/*
	 * If we are in kernel mode at this point, we have no context to
	 * handle this fault with.
	 */
	if (!user_mode(regs))
		goto no_context;

	if (fault & VM_FAULT_OOM) {
		/*
		 * We ran out of memory, call the OOM killer, and return to
		 * userspace (which will retry the fault, or kill us if we got
		 * oom-killed).
		 */
		pagefault_out_of_memory();
		return 0;
	}

	inf = esr_to_fault_info(esr);
	set_thread_esr(addr, esr);
	if (fault & VM_FAULT_SIGBUS) {
		/*
		 * We had some memory, but were unable to successfully fix up
		 * this page fault.
		 */
		arm64_force_sig_fault(SIGBUS, BUS_ADRERR, (void __user *)addr,
				      inf->name);
	} else if (fault & (VM_FAULT_HWPOISON_LARGE | VM_FAULT_HWPOISON)) {
		unsigned int lsb;

		lsb = PAGE_SHIFT;
		if (fault & VM_FAULT_HWPOISON_LARGE)
			lsb = hstate_index_to_shift(VM_FAULT_GET_HINDEX(fault));

		arm64_force_sig_mceerr(BUS_MCEERR_AR, (void __user *)addr, lsb,
				       inf->name);
	} else {
		/*
		 * Something tried to access memory that isn't in our memory
		 * map.
		 */
		arm64_force_sig_fault(SIGSEGV,
				      fault == VM_FAULT_BADACCESS ? SEGV_ACCERR : SEGV_MAPERR,
				      (void __user *)addr,
				      inf->name);
	}

	return 0;

no_context:
	__do_kernel_fault(addr, esr, regs);
	return 0;
}
```

分析__do_page_fault()函数实现，arch/arm64/mm/fault.c中。
```C
static vm_fault_t __do_page_fault(struct mm_struct *mm, unsigned long addr,
			   unsigned int mm_flags, unsigned long vm_flags,
			   struct task_struct *tsk)
{
	struct vm_area_struct *vma;
	vm_fault_t fault;

    // find_vma,通过失效地址返回vma.
	vma = find_vma(mm, addr);
	fault = VM_FAULT_BADMAP;
	if (unlikely(!vma))
		goto out;
	if (unlikely(vma->vm_start > addr))
		goto check_stack;

	/*
	 * Ok, we have a good vm_area for this memory access, so we can handle
	 * it.
	 */
good_area:
	/*
	 * Check that the permissions on the VMA allow for the fault which
	 * occurred.
	 */
	if (!(vma->vm_flags & vm_flags)) {
		fault = VM_FAULT_BADACCESS;
		goto out;
	}

	return handle_mm_fault(vma, addr & PAGE_MASK, mm_flags);

check_stack:
	if (vma->vm_flags & VM_GROWSDOWN && !expand_stack(vma, addr))
		goto good_area;
out:
	return fault;
}

```

```C
/*
 * By the time we get here, we already hold the mm semaphore
 *
 * The mmap_sem may have been released depending on flags and our
 * return value.  See filemap_fault() and __lock_page_or_retry().
 */
 /*
 * vma: 缺页异常时进程地址空间
 * address: 缺页异常时虚拟地址，页面大小对齐的
 * flags: 内存相关的标志位
 */
vm_fault_t handle_mm_fault(struct vm_area_struct *vma, unsigned long address,
		unsigned int flags)
{
	vm_fault_t ret;

	__set_current_state(TASK_RUNNING);

	count_vm_event(PGFAULT);
	count_memcg_event_mm(vma->vm_mm, PGFAULT);

	/* do counter updates before entering really critical section. */
	check_sync_rss_stat(current);

	if (!arch_vma_access_permitted(vma, flags & FAULT_FLAG_WRITE,
					    flags & FAULT_FLAG_INSTRUCTION,
					    flags & FAULT_FLAG_REMOTE))
		return VM_FAULT_SIGSEGV;

	/*
	 * Enable the memcg OOM handling for faults triggered in user
	 * space.  Kernel faults are handled more gracefully.
	 */
	if (flags & FAULT_FLAG_USER)
		mem_cgroup_enter_user_fault();

	if (unlikely(is_vm_hugetlb_page(vma)))
		ret = hugetlb_fault(vma->vm_mm, vma, address, flags);
	else
		ret = __handle_mm_fault(vma, address, flags);

	if (flags & FAULT_FLAG_USER) {
		mem_cgroup_exit_user_fault();
		/*
		 * The task may have entered a memcg OOM situation but
		 * if the allocation error was handled gracefully (no
		 * VM_FAULT_OOM), there is no need to kill anything.
		 * Just clean up the OOM state peacefully.
		 */
		if (task_in_memcg_oom(current) && !(ret & VM_FAULT_OOM))
			mem_cgroup_oom_synchronize(false);
	}

	return ret;
}

```

```C
/*
 * By the time we get here, we already hold the mm semaphore
 *
 * The mmap_sem may have been released depending on flags and our
 * return value.  See filemap_fault() and __lock_page_or_retry().
 */
static vm_fault_t __handle_mm_fault(struct vm_area_struct *vma,
		unsigned long address, unsigned int flags)
{
	struct vm_fault vmf = {
		.vma = vma,
		.address = address & PAGE_MASK,
		.flags = flags,
		.pgoff = linear_page_index(vma, address),
		.gfp_mask = __get_fault_gfp_mask(vma),
	};
	unsigned int dirty = flags & FAULT_FLAG_WRITE;
	struct mm_struct *mm = vma->vm_mm;
	pgd_t *pgd;
	p4d_t *p4d;
	vm_fault_t ret;

    //计算pgd页表项
	pgd = pgd_offset(mm, address);
	//计算p4d页表项
	p4d = p4d_alloc(mm, pgd, address);
	if (!p4d)
		return VM_FAULT_OOM;

    //计算pud页表项
	vmf.pud = pud_alloc(mm, p4d, address);
	if (!vmf.pud)
		return VM_FAULT_OOM;
	if (pud_none(*vmf.pud) && __transparent_hugepage_enabled(vma)) {
		ret = create_huge_pud(&vmf);
		if (!(ret & VM_FAULT_FALLBACK))
			return ret;
	} else {
		pud_t orig_pud = *vmf.pud;

		barrier();
		if (pud_trans_huge(orig_pud) || pud_devmap(orig_pud)) {

			/* NUMA case for anonymous PUDs would go here */

			if (dirty && !pud_write(orig_pud)) {
				ret = wp_huge_pud(&vmf, orig_pud);
				if (!(ret & VM_FAULT_FALLBACK))
					return ret;
			} else {
				huge_pud_set_accessed(&vmf, orig_pud);
				return 0;
			}
		}
	}

    //计算pmd页表项
	vmf.pmd = pmd_alloc(mm, vmf.pud, address);
	if (!vmf.pmd)
		return VM_FAULT_OOM;
	if (pmd_none(*vmf.pmd) && __transparent_hugepage_enabled(vma)) {
		ret = create_huge_pmd(&vmf);
		if (!(ret & VM_FAULT_FALLBACK))
			return ret;
	} else {
		pmd_t orig_pmd = *vmf.pmd;

		barrier();
		if (unlikely(is_swap_pmd(orig_pmd))) {
			VM_BUG_ON(thp_migration_supported() &&
					  !is_pmd_migration_entry(orig_pmd));
			if (is_pmd_migration_entry(orig_pmd))
				pmd_migration_entry_wait(mm, vmf.pmd);
			return 0;
		}
		if (pmd_trans_huge(orig_pmd) || pmd_devmap(orig_pmd)) {
			if (pmd_protnone(orig_pmd) && vma_is_accessible(vma))
				return do_huge_pmd_numa_page(&vmf, orig_pmd);

			if (dirty && !pmd_write(orig_pmd)) {
				ret = wp_huge_pmd(&vmf, orig_pmd);
				if (!(ret & VM_FAULT_FALLBACK))
					return ret;
			} else {
				huge_pmd_set_accessed(&vmf, orig_pmd);
				return 0;
			}
		}
	}

	return handle_pte_fault(&vmf);
}
```

```C
static vm_fault_t handle_pte_fault(struct vm_fault *vmf)
{
	pte_t entry;
    //从handle_mm_fault传递过来的vmf只有pmd，因此先判断pmd是否为空
	if (unlikely(pmd_none(*vmf->pmd))) {
		/*
		 * Leave __pte_alloc() until later: because vm_ops->fault may
		 * want to allocate huge page, and if we expose page table
		 * for an instant, it will be difficult to retract from
		 * concurrent faults and from rmap lookups.
		 */
		vmf->pte = NULL;
	} else {
		/* See comment in pte_alloc_one_map() */
		if (pmd_devmap_trans_unstable(vmf->pmd))
			return 0;
		/*
		 * A regular pmd is established and it can't morph into a huge
		 * pmd from under us anymore at this point because we hold the
		 * mmap_sem read mode and khugepaged takes it in write mode.
		 * So now it's safe to run pte_offset_map().
		 */
		// 由pmd和address计算出PTE，并读取PTE的内容到vmf->orig_pte
        // arch/arm64/include/asm/pgtable.h
		// #define pte_offset_map(dir,addr)	pte_offset_kernel((dir), (addr))
		vmf->pte = pte_offset_map(vmf->pmd, vmf->address);
		vmf->orig_pte = *vmf->pte;

		/*
		 * some architectures can have larger ptes than wordsize,
		 * e.g.ppc44x-defconfig has CONFIG_PTE_64BIT=y and
		 * CONFIG_32BIT=y, so READ_ONCE cannot guarantee atomic
		 * accesses.  The code below just needs a consistent view
		 * for the ifs and we later double check anyway with the
		 * ptl lock held. So here a barrier will do.
		 */
		// 有的处理器架构的PET会大于word size，所以READ_ONCE和ACCESS_ONCE并不能保证访问原子性
		barrier();
		/*
		 * 为何判断orig_pte的页表项是否为空：
		 * 场景如下：线程1调用ptep_get_and_clear()函数清空PTE，需要线程2读这个页面，线程2运行
		 * 到此处后发现PTE内容为空，因此会继续往下走缺页异常流程。
		 */
		if (pte_none(vmf->orig_pte)) {
			pte_unmap(vmf->pte);
			vmf->pte = NULL;
		}
	}

    // 页表项还没建立，两种可能，一是页表项还没建立，而是页表项的内容被清空，即调用线程1调用ptep_get_and_clear
	if (!vmf->pte) {
	    // vma->vm_ops没有实现方法集，则为匿名映射，否则，就是文件映射。
		if (vma_is_anonymous(vmf->vma))
			return do_anonymous_page(vmf);
		else
			return do_fault(vmf);
	}
    // PTE已经分配，内容不是空，但是pte_present()为0，说明页面不在内存中，即PTE还未映射到物理页面
    // 调用do_swap_page从交换分区中读回页面
	if (!pte_present(vmf->orig_pte))
		return do_swap_page(vmf);

    //调用pte_protone说明页面被设置为NUMA调度页面
	if (pte_protnone(vmf->orig_pte) && vma_is_accessible(vmf->vma))
		return do_numa_page(vmf);

    // 获取进程描述符mm中定义的一个自旋锁
	vmf->ptl = pte_lockptr(vmf->vma->vm_mm, vmf->pmd);
	spin_lock(vmf->ptl);
	entry = vmf->orig_pte;
	
	//判断这段时间内进PTE是否被修改了
	if (unlikely(!pte_same(*vmf->pte, entry)))
		goto unlock;
	//do_page_fault(arch/arm64/mm/fault.c)，通过ESR中的WnR字段（ESR_ELx_WNR）判断处理器是否
	//因为写内容而触发异常
	if (vmf->flags & FAULT_FLAG_WRITE) {
	    // 如果处理器写内存触发异常，且PTE是只读属性。进行写时复制的缺页中断
		if (!pte_write(entry))
			return do_wp_page(vmf);
		// 如果pte是可写属性，则通过如下函数设置PTE_DIRTY
		entry = pte_mkdirty(entry);
	}
	entry = pte_mkyoung(entry);// 对于arm64，设置PTE_AF位
	
	// ptep_set_access_flags判断PTE内容是否发生变化
	// 若改变，则需要将把新的内容写入到PTE，并刷新对应的TLB和高速缓存。
	if (ptep_set_access_flags(vmf->vma, vmf->address, vmf->pte, entry,
				vmf->flags & FAULT_FLAG_WRITE)) {
		update_mmu_cache(vmf->vma, vmf->address, vmf->pte);
	} else {
		/*
		 * This is needed only for protection faults but the arch code
		 * is not yet telling us if this is a protection fault or not.
		 * This still avoids useless tlb flushes for .text page faults
		 * with threads.
		 */
		if (vmf->flags & FAULT_FLAG_WRITE)
			flush_tlb_fix_spurious_fault(vmf->vma, vmf->address);
	}
unlock:
	pte_unmap_unlock(vmf->pte, vmf->ptl);
	return 0;
}
```

文件映射缺页中断
<mm/memory.c>
static vm_fault_t do_fault(struct vm_fault *vmf)
```C
/*
 * We enter with non-exclusive mmap_sem (to exclude vma changes,
 * but allow concurrent faults).
 * The mmap_sem may have been released depending on flags and our
 * return value.  See filemap_fault() and __lock_page_or_retry().
 */
static vm_fault_t do_fault(struct vm_fault *vmf)
{
	struct vm_area_struct *vma = vmf->vma;
	vm_fault_t ret;

	/*
	 * The VMA was not fully populated on mmap() or missing VM_DONTEXPAND
	 */
	if (!vma->vm_ops->fault) {
		/*
		 * If we find a migration pmd entry or a none pmd entry, which
		 * should never happen, return SIGBUS
		 */
		if (unlikely(!pmd_present(*vmf->pmd)))
			ret = VM_FAULT_SIGBUS;
		else {
			vmf->pte = pte_offset_map_lock(vmf->vma->vm_mm,
						       vmf->pmd,
						       vmf->address,
						       &vmf->ptl);
			/*
			 * Make sure this is not a temporary clearing of pte
			 * by holding ptl and checking again. A R/M/W update
			 * of pte involves: take ptl, clearing the pte so that
			 * we don't have concurrent modification by hardware
			 * followed by an update.
			 */
			if (unlikely(pte_none(*vmf->pte)))
				ret = VM_FAULT_SIGBUS;
			else
				ret = VM_FAULT_NOPAGE;

			pte_unmap_unlock(vmf->pte, vmf->ptl);
		}
	} else if (!(vmf->flags & FAULT_FLAG_WRITE))
		ret = do_read_fault(vmf);
	else if (!(vma->vm_flags & VM_SHARED))
		ret = do_cow_fault(vmf);
	else
		ret = do_shared_fault(vmf);

	/* preallocated pagetable is unused: free it */
	if (vmf->prealloc_pte) {
		pte_free(vma->vm_mm, vmf->prealloc_pte);
		vmf->prealloc_pte = NULL;
	}
	return ret;
}
```

do_shared_fault处理，用于共享文件映射发生缺页异常
```C
static vm_fault_t do_shared_fault(struct vm_fault *vmf)
{
	struct vm_area_struct *vma = vmf->vma;
	vm_fault_t ret, tmp;

    //调用_do_fault(),并通过vma->vm_ops->fault()回调函数读取文件内容到vmf->page中
	ret = __do_fault(vmf);
	if (unlikely(ret & (VM_FAULT_ERROR | VM_FAULT_NOPAGE | VM_FAULT_RETRY)))
		return ret;

	/*
	 * Check if the backing address space wants to know that the page is
	 * about to become writable
	 */
	// 判断为如果vm_ops定义了page_mkwrite方法，调用page_mkwrite通知进程地址空间，
	// 页面变成可写. 若一个页面变成可写，进程可能需要等待这个页面内容会写成功。
    // 根据Trace观察，__do_fault返回后，在Arm64中并未执行到这里。在X86_64环境中执行到了这里，触发do_page_mkwrite.
    // arm64中，__do_fault后直接去了finish_fault
	if (vma->vm_ops->page_mkwrite) {
		unlock_page(vmf->page);
		tmp = do_page_mkwrite(vmf);
		if (unlikely(!tmp ||
				(tmp & (VM_FAULT_ERROR | VM_FAULT_NOPAGE)))) {
			put_page(vmf->page);
			return tmp;
		}
	}

    // 使用finish_fault，使用vmf->page页面制作一个pte，并将其设置到物理页表中。
    // 并把vmf->page添加到文件页面的RMAP机制中
	ret |= finish_fault(vmf);
	if (unlikely(ret & (VM_FAULT_ERROR | VM_FAULT_NOPAGE |
					VM_FAULT_RETRY))) {
		unlock_page(vmf->page);
		put_page(vmf->page);
		return ret;
	}
    // 设置为脏页
	fault_dirty_shared_page(vma, vmf->page);
	return ret;
}

```

```c
static vm_fault_t do_read_fault(struct vm_fault *vmf)
{
	struct vm_area_struct *vma = vmf->vma;
	vm_fault_t ret = 0;

	/*
	 * Let's call ->map_pages() first and use ->fault() as fallback
	 * if page by the offset is not ready to be mapped (cold cache or
	 * something).
	 */
	if (vma->vm_ops->map_pages && fault_around_bytes >> PAGE_SHIFT > 1) {
		ret = do_fault_around(vmf);
		if (ret)
			return ret;
	}

	ret = __do_fault(vmf);
	if (unlikely(ret & (VM_FAULT_ERROR | VM_FAULT_NOPAGE | VM_FAULT_RETRY)))
		return ret;

	ret |= finish_fault(vmf);
	unlock_page(vmf->page);
	if (unlikely(ret & (VM_FAULT_ERROR | VM_FAULT_NOPAGE | VM_FAULT_RETRY)))
		put_page(vmf->page);
	return ret;
}

```

```C
/*
 * The mmap_sem must have been held on entry, and may have been
 * released depending on flags and vma->vm_ops->fault() return value.
 * See filemap_fault() and __lock_page_retry().
 */
static vm_fault_t __do_fault(struct vm_fault *vmf)
{
	struct vm_area_struct *vma = vmf->vma;
	vm_fault_t ret;

	/*
	 * Preallocate pte before we take page_lock because this might lead to
	 * deadlocks for memcg reclaim which waits for pages under writeback:
	 *				lock_page(A)
	 *				SetPageWriteback(A)
	 *				unlock_page(A)
	 * lock_page(B)
	 *				lock_page(B)
	 * pte_alloc_pne
	 *   shrink_page_list
	 *     wait_on_page_writeback(A)
	 *				SetPageWriteback(B)
	 *				unlock_page(B)
	 *				# flush A, B to clear the writeback
	 */
	if (pmd_none(*vmf->pmd) && !vmf->prealloc_pte) {
		vmf->prealloc_pte = pte_alloc_one(vmf->vma->vm_mm);
		if (!vmf->prealloc_pte)
			return VM_FAULT_OOM;
		smp_wmb(); /* See comment in __pte_alloc() */
	}

	ret = vma->vm_ops->fault(vmf);
	if (unlikely(ret & (VM_FAULT_ERROR | VM_FAULT_NOPAGE | VM_FAULT_RETRY |
			    VM_FAULT_DONE_COW)))
		return ret;

	if (unlikely(PageHWPoison(vmf->page))) {
		if (ret & VM_FAULT_LOCKED)
			unlock_page(vmf->page);
		put_page(vmf->page);
		vmf->page = NULL;
		return VM_FAULT_HWPOISON;
	}

	if (unlikely(!(ret & VM_FAULT_LOCKED)))
		lock_page(vmf->page);
	else
		VM_BUG_ON_PAGE(!PageLocked(vmf->page), vmf->page);

	return ret;
}
```

do_page_mkwrite函数封装
```C
/*
 * Notify the address space that the page is about to become writable so that
 * it can prohibit this or wait for the page to get into an appropriate state.
 *
 * We do this without the lock held, so that it can sleep if it needs to.
 */
static vm_fault_t do_page_mkwrite(struct vm_fault *vmf)
{
	vm_fault_t ret;
	struct page *page = vmf->page;
	unsigned int old_flags = vmf->flags;

	vmf->flags = FAULT_FLAG_WRITE|FAULT_FLAG_MKWRITE;

	ret = vmf->vma->vm_ops->page_mkwrite(vmf);
	/* Restore original flags so that caller is not surprised */
	vmf->flags = old_flags;
	if (unlikely(ret & (VM_FAULT_ERROR | VM_FAULT_NOPAGE)))
		return ret;
	// 返回值不是VM_FAULT_LOCKED
	if (unlikely(!(ret & VM_FAULT_LOCKED))) {
		lock_page(page);
		if (!page->mapping) {
			unlock_page(page);
			return 0; /* retry */
		}
		ret |= VM_FAULT_LOCKED;
	} else
		VM_BUG_ON_PAGE(!PageLocked(page), page);
	return ret;
}
```

include/mm.h
```C
#define VM_FAULT_NOPAGE	0x0100	/* ->fault installed the pte, not return page */
#define VM_FAULT_LOCKED	0x0200	/* ->fault locked the returned page */
#define VM_FAULT_RETRY	0x0400	/* ->fault blocked, must retry */

/*
 * vm_fault is filled by the the pagefault handler and passed to the vma's
 * ->fault function. The vma's ->fault is responsible for returning a bitmask
 * of VM_FAULT_xxx flags that give details about how the fault was handled.
 *
 * MM layer fills up gfp_mask for page allocations but fault handler might
 * alter it if its implementation requires a different allocation context.
 *
 * pgoff should be used in favour of virtual_address, if possible.
 */
struct vm_fault {
	struct vm_area_struct *vma;	/* Target VMA */
	unsigned int flags;		/* FAULT_FLAG_xxx flags */
	gfp_t gfp_mask;			/* gfp mask to be used for allocations */
	pgoff_t pgoff;			/* Logical page offset based on vma */
	unsigned long address;		/* Faulting virtual address */
	pmd_t *pmd;			/* Pointer to pmd entry matching
					 * the 'address' */
	pud_t *pud;			/* Pointer to pud entry matching
					 * the 'address'
					 */
	pte_t orig_pte;			/* Value of PTE at the time of fault */

	struct page *cow_page;		/* Page handler may use for COW fault */
	struct mem_cgroup *memcg;	/* Cgroup cow_page belongs to */
	struct page *page;		/* ->fault handlers should return a
					 * page here, unless VM_FAULT_NOPAGE
					 * is set (which is also implied by
					 * VM_FAULT_ERROR).
					 */
	/* These three entries are valid only while holding ptl lock */
	pte_t *pte;			/* Pointer to pte entry matching
					 * the 'address'. NULL if the page
					 * table hasn't been allocated.
					 */
	spinlock_t *ptl;		/* Page table lock.
					 * Protects pte page table if 'pte'
					 * is not NULL, otherwise pmd.
					 */
	pgtable_t prealloc_pte;		/* Pre-allocated pte page table.
					 * vm_ops->map_pages() calls
					 * alloc_set_pte() from atomic context.
					 * do_fault_around() pre-allocates
					 * page table to avoid allocation from
					 * atomic context.
					 */
};

/* page entry size for vm->huge_fault() */
enum page_entry_size {
	PE_SIZE_PTE = 0,
	PE_SIZE_PMD,
	PE_SIZE_PUD,
};
```

include/linux/mm_types.h

```c
/*
 * This struct defines a memory VMM memory area. There is one of these
 * per VM-area/task.  A VM area is any part of the process virtual memory
 * space that has a special rule for the page-fault handlers (ie a shared
 * library, the executable area etc).
 */
struct vm_area_struct {
	/* The first cache line has the info for VMA tree walking. */

	unsigned long vm_start;		/* Our start address within vm_mm. */
	unsigned long vm_end;		/* The first byte after our end address
					   within vm_mm. */

	/* linked list of VM areas per task, sorted by address */
	struct vm_area_struct *vm_next, *vm_prev;

	struct rb_node vm_rb;

	/*
	 * Largest free memory gap in bytes to the left of this VMA.
	 * Either between this VMA and vma->vm_prev, or between one of the
	 * VMAs below us in the VMA rbtree and its ->vm_prev. This helps
	 * get_unmapped_area find a free area of the right size.
	 */
	unsigned long rb_subtree_gap;

	/* Second cache line starts here. */

	struct mm_struct *vm_mm;	/* The address space we belong to. */
	pgprot_t vm_page_prot;		/* Access permissions of this VMA. */
	unsigned long vm_flags;		/* Flags, see mm.h. */

	/*
	 * For areas with an address space and backing store,
	 * linkage into the address_space->i_mmap interval tree.
	 */
	struct {
		struct rb_node rb;
		unsigned long rb_subtree_last;
	} shared;

	/*
	 * A file's MAP_PRIVATE vma can be in both i_mmap tree and anon_vma
	 * list, after a COW of one of the file pages.	A MAP_SHARED vma
	 * can only be in the i_mmap tree.  An anonymous MAP_PRIVATE, stack
	 * or brk vma (with NULL file) can only be in an anon_vma list.
	 */
	struct list_head anon_vma_chain; /* Serialized by mmap_sem &
					  * page_table_lock */
	struct anon_vma *anon_vma;	/* Serialized by page_table_lock */

	/* Function pointers to deal with this struct. */
	const struct vm_operations_struct *vm_ops;

	/* Information about our backing store: */
	unsigned long vm_pgoff;		/* Offset (within vm_file) in PAGE_SIZE
					   units */
	struct file * vm_file;		/* File we map to (can be NULL). */
	void * vm_private_data;		/* was vm_pte (shared mem) */

	atomic_long_t swap_readahead_info;
#ifndef CONFIG_MMU
	struct vm_region *vm_region;	/* NOMMU mapping region */
#endif
#ifdef CONFIG_NUMA
	struct mempolicy *vm_policy;	/* NUMA policy for the VMA */
#endif
	struct vm_userfaultfd_ctx vm_userfaultfd_ctx;
} __randomize_layout;
```



Reference
https://pzh2386034.github.io/Black-Jack/linux-memory/2019/09/15/ARM64%E5%86%85%E5%AD%98%E7%AE%A1%E7%90%86%E5%8D%81%E4%BA%94-do_page_fault%E7%BC%BA%E9%A1%B5%E4%B8%AD%E6%96%AD/