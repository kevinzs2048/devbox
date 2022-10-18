# Pmem No Flush on Arm64


```angular2html
pmem_init(void)
pmem2_arch_init(&info);

	char *e = os_getenv("PMEM_NO_FLUSH");
	if (e && (strcmp(e, "1") == 0)) {
		flush = 0;
		LOG(3, "Forced not flushing CPU_cache");

	Funcs.deep_flush = info.flush;
	if (flush) {
.......
	} else {
		Funcs.memmove_nodrain = info.memmove_nodrain_eadr;  //都是X86的逻辑，导致这个地方为NULL
		Funcs.memset_nodrain = info.memset_nodrain_eadr;    //都是X86的逻辑，导致这个地方为NULL
		Funcs.flush = flush_empty;
		Funcs.fence = info.fence;
	}

	char *ptr = os_getenv("PMEM_NO_GENERIC_MEMCPY");
	long long no_generic = 0;
	if (ptr)
		no_generic = atoll(ptr);
// 为NULL之后，会调用到这里
	if (info.memmove_nodrain == NULL) {
		if (no_generic) {
			Funcs.memmove_nodrain = memmove_nodrain_libc;
			LOG(3, "using libc memmove");
		} else {
			Funcs.memmove_nodrain = memmove_nodrain_generic;
			LOG(3, "using generic memmove");
		}
	} else {
		Funcs.memmove_nodrain = info.memmove_nodrain;
	}

	if (info.memset_nodrain == NULL) {
		if (no_generic) {
			Funcs.memset_nodrain = memset_nodrain_libc;
			LOG(3, "using libc memset");
		} else {
			Funcs.memset_nodrain = memset_nodrain_generic;
			LOG(3, "using generic memset");
		}
	} else {
		Funcs.memset_nodrain = info.memset_nodrain;
	}


```

