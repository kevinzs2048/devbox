# Nodemap create issue on 64K page Analysis
## Test ENV
5 machines, 2 MDS, 2 Clients, and 1 OST
Sanity-sec test 7

## Reproduce steps
Ready nodes: ['192.168.1.223', '192.168.1.35', '192.168.1.123', '192.168.1.13', '192.168.1.207']
echo 8 >  /proc/sys/kernel/printk

```angular2html
/usr/lib64/lustre/tests/auster -f multinode -rvs sanity-sec --only 7
```

Manual step:
On Lustre 01:
```angular2html
/usr/lib64/lustre/tests/llmount.sh -f /usr/lib64/lustre/tests/cfg/multinode.sh
```

On Lustre 03:
```angular2html
rm lustre.log* -rf
lctl clear
lctl set_param debug=-1
lctl show mgc
lctl show mgs
lctl debug_daemon start /root/lustre.log.bin 5000
lctl mark "============TEST start============="


/usr/sbin/lctl nodemap_add 45553_0
/usr/sbin/lctl get_param nodemap.45553_0.id
/usr/sbin/lctl nodemap_add 45553_1
/usr/sbin/lctl get_param nodemap.45553_1.id
lctl mark "============TEST: end============="
/usr/sbin/lctl debug_daemon stop
lctl debug_file lustre.log.bin lustre.64k.log03
```

On Lustre 05:
```angular2html
rm lustre.log* -rf
lctl clear
lctl set_param debug=-1
lctl show mgc
lctl show mgs
lctl debug_daemon start /root/lustre.log.bin 5000
lctl mark "============TEST start============="
/usr/sbin/lctl get_param nodemap.45553_0.id
/usr/sbin/lctl get_param nodemap.45553_1.id

lctl mark "============TEST: end============="
/usr/sbin/lctl debug_daemon stop
lctl debug_file lustre.log.bin lustre.64k.log05
```

## Code Analysis
mgc_request.c
```C
enum {
	CONFIG_READ_NRPAGES_INIT = 1 << (20 - PAGE_SHIFT),
        CONFIG_READ_NRPAGES      = 4
};

struct mgs_config_body {
	char     mcb_name[MTI_NAME_MAXLEN]; /* logname */
	__u64    mcb_offset;    /* next index of config log to request */
	__u16    mcb_type;      /* type of log: MGS_CFG_T_[CONFIG|RECOVER] */
	__u8     mcb_nm_cur_pass;
	__u8     mcb_bits;      /* bits unit size of config log */
	__u32    mcb_units;     /* # of units for bulk transfer */
};

mgc_process_recover_nodemap_log{
    nrpages = CONFIG_READ_NRPAGES_INIT
}
OBD_ALLOC_PTR_ARRAY_LARGE(pages, nrpages);
for (i = 0; i < nrpages; i++) {
    pages[i] = alloc_page(GFP_KERNEL);
    if (pages[i] == NULL)
        GOTO(out, rc = -ENOMEM);
}

body = req_capsule_client_get(&req->rq_pill, &RMF_MGS_CONFIG_BODY);
//body is the struct mgs_config_body

// allocate bulk transfer descriptor
/* allocate bulk transfer descriptor */
desc = ptlrpc_prep_bulk_imp(req, nrpages, 1,
                PTLRPC_BULK_PUT_SINK,
                MGS_BULK_PORTAL,
                &ptlrpc_bulk_kiov_pin_ops);
desc===> struct ptlrpc_bulk_desc {}
 * Definition of bulk descriptor.
 * Bulks are special "Two phase" RPCs where initial request message
 * is sent first and it is followed bt a transfer (o receiving) of a large
 * amount of data to be settled into pages referenced from the bulk descriptors.
 * Bulks transfers (the actual data following the small requests) are done
 * on separate LNet portals.
 * In lustre we use bulk transfers for READ and WRITE transfers from/to OSTs.
 *  Another user is readpage for MDT.

	for (i = 0; i < nrpages; i++)
		desc->bd_frag_ops->add_kiov_frag(desc, pages[i], 0,
						 PAGE_SIZE);
desc->bd_frag_ops:
const struct ptlrpc_bulk_frag_ops *bd_frag_ops;
    /**
	 * Add a page \a page to the bulk descriptor \a desc
	 * Data to transfer in the page starts at offset \a pageoffset and
	 * amount of data to transfer from the page is \a len
	 */
	void (*add_kiov_frag)(struct ptlrpc_bulk_desc *desc,
			      struct page *page, int pageoffset, int len);

ptlrpc_bulk_kiov_pin_ops===> -> ptlrpc_prep_bulk_imp has already use this args.
static void ptlrpc_prep_bulk_page_pin(struct ptlrpc_bulk_desc *desc,
				      struct page *page, int pageoffset,
				      int len)
{
	__ptlrpc_prep_bulk_page(desc, page, pageoffset, len, 1);
}
				     
__ptlrpc_prep_bulk_page(desc, page, pageoffset, len, 1); 
The function above is defined at lustre/ptlrpc/client.c


```

```
/**
 * Trivial wrapper around __req_capsule_get(), that returns the PTLRPC reply
 * buffer corresponding to the given RMF (\a field) of a \a pill.
 */
void *req_capsule_server_get(struct req_capsule *pill,
                             const struct req_msg_field *field)
{
	return __req_capsule_get(pill, field, RCL_SERVER, NULL, false);
}
```

The error occurs at:
```
这里Page在之前的操作：
    struct page **pages = NULL;
	OBD_ALLOC_PTR_ARRAY_LARGE(pages, nrpages);
	if (pages == NULL)
		GOTO(out, rc = -ENOMEM);

	for (i = 0; i < nrpages; i++) {
		pages[i] = alloc_page(GFP_KERNEL);
		if (pages[i] == NULL)
			GOTO(out, rc = -ENOMEM);
	}

	for (i = 0; i < nrpages && ealen > 0; i++) {
		int rc2;
		union lu_page	*ptr;

		ptr = kmap(pages[i]);
		if (cld_is_nodemap(cld)) {
		    // 实际运行到这里
            rc2 = nodemap_process_idx_pages(new_config, ptr,
                                            &recent_nodemap);
        }
		else {
			rc2 = mgc_apply_recover_logs(obd, cld, res->mcr_offset,
						     ptr,
						     min_t(int, ealen,
							   PAGE_SIZE),
						     mne_swab);
        }
		kunmap(pages[i]);
		if (rc2 < 0) {
			CWARN("%s: error processing %s log %s: rc = %d\n",
			      obd->obd_name,
			      cld_is_nodemap(cld) ? "nodemap" : "recovery",
			      cld->cld_logname,
			      rc2);
			GOTO(out, rc = rc2);
		}

		ealen -= PAGE_SIZE;
	}

/* Gather all possible type associated with a 4KB container */
union lu_page {
	struct lu_dirpage	lp_dir; /* for MDS_READPAGE */
	struct lu_idxpage	lp_idx; /* for OBD_IDX_READ */
	char			lp_array[LU_PAGE_SIZE];
};

```

重点分析：
```c

int nodemap_process_idx_pages(struct nodemap_config *config, union lu_page *lip,
			      struct lu_nodemap **recent_nodemap)
{
	struct nodemap_key *key;
	union nodemap_rec *rec;
	char *entry;
	int j;
	int k;
	int rc = 0;
	int size = dt_nodemap_features.dif_keysize_max +
		   dt_nodemap_features.dif_recsize_max;
	ENTRY;
    CWARN("================nodemap_process_idx_pages========%x=======\n", lip->lp_idx.lip_magic);

	for (j = 0; j < LU_PAGE_COUNT; j++) {
		if (lip->lp_idx.lip_magic != LIP_MAGIC){
            CERROR("================nodemap_process_idx_pages: lip->lp_idx.lip_magic != LIP_MAGIC===============\n");
            return -EINVAL;
        }
        CWARN("================nodemap_process_idx_pages lip->lp_idx.lip_nr: %d===============\n", lip->lp_idx.lip_nr);
		/* get and process keys and records from page */
		for (k = 0; k < lip->lp_idx.lip_nr; k++) {
			entry = lip->lp_idx.lip_entries + k * size;
			key = (struct nodemap_key *)entry;

			entry += dt_nodemap_features.dif_keysize_max;
			rec = (union nodemap_rec *)entry;
            CWARN("================nodemap_process_idx_pages nodemap_process_keyrec===============\n");
			rc = nodemap_process_keyrec(config, key, rec,
						    recent_nodemap);
			if (rc < 0){
                CWARN("================nodemap_process_keyrec Error %d===============\n", rc);
                return rc;
            }

		}
		lip++;
	}
    CWARN("================nodemap_process_idx_pages EXIT===============\n");
	EXIT;
	return 0;
}
```
目前行为：lip++后再次进入循环，检查lip_magic 报错。
重点分析：
1. 用到的结构体
```c
#define LIP_HDR_SIZE (offsetof(struct lu_idxpage, lip_entries))

/* Gather all possible type associated with a 4KB container */
union lu_page {
	struct lu_dirpage	lp_dir; /* for MDS_READPAGE */
	struct lu_idxpage	lp_idx; /* for OBD_IDX_READ */
	char			lp_array[LU_PAGE_SIZE];
};

#define LIP_MAGIC 0x8A6D6B6C

/* 4KB (= LU_PAGE_SIZE) container gathering key/record pairs */
struct lu_idxpage {
	/* 16-byte header */
	__u32	lip_magic;
	__u16	lip_flags;
	__u16	lip_nr;   /* number of entries in the container */
	__u64	lip_pad0; /* additional padding for future use */

	/* key/record pairs are stored in the remaining 4080 bytes.
	 * depending upon the flags in idx_info::ii_flags, each key/record
	 * pair might be preceded by:
	 * - a hash value
	 * - the key size (II_FL_VARKEY is set)
	 * - the record size (II_FL_VARREC is set)
	 *
	 * For the time being, we only support fixed-size key & record. */
	char	lip_entries[0];
};
================nodemap_process_idx_pages lip->lp_idx.lip_nr: 2===============

 * This is the directory page size packed in MDS_READPAGE RPC.
 * It's different than PAGE_SIZE because the client needs to
 * access the struct lu_dirpage header packed at the beginning of
 * the "page" and without this there isn't any way to know find the
 * lu_dirpage header is if client and server PAGE_SIZE differ.
 */
#define LU_PAGE_SHIFT 12
#define LU_PAGE_SIZE  (1UL << LU_PAGE_SHIFT)
#define LU_PAGE_MASK  (~(LU_PAGE_SIZE - 1))

#define LU_PAGE_COUNT (1 << (PAGE_SHIFT - LU_PAGE_SHIFT))
64K page情况下，LU_PAGE_COUNT=16. 4K下为1
上面的定义LU_PAGE_SIZE，实际为一个物理页对应的Lustre 4K页面个数。
```

### MGS端相关分析
int dt_index_walk(const struct lu_env *env, struct dt_object *obj,
		  const struct lu_rdpg *rdpg, dt_index_page_build_t filler,
		  void *arg)
```
	/*
	 * Fill containers one after the other. There might be multiple
	 * containers per physical page.
	 *
	 * At this point and across for-loop:
	 *  rc == 0 -> ok, proceed.
	 *  rc >  0 -> end of index.
	 *  rc <  0 -> error.
	 */
	 //这里，大循环指的是每个物理page。结束循环依据是nob > 0. nob根据前文得知，为传输的bytes数目，由mdt提供。
	 //因此，只要nob>0,此处就会执行。nob会在每次进入循环后，根据LU_PAGE_SIZE再计算。
	for (pageidx = 0; rc == 0 && nob > 0; pageidx++) {
		union lu_page	*lp;
		int		 i;

		LASSERT(pageidx < rdpg->rp_npages);
		// 映射一个物理page
		lp = kmap(rdpg->rp_pages[pageidx]);

		/* fill lu pages */
		// 此处，是给每个LU_PAGE(4K)添加内容。
		// 如果第一次执行完，会进行lp++，到下一个4k区间。但这里逻辑，
		for (i = 0; i < LU_PAGE_COUNT; i++, lp++, nob -= LU_PAGE_SIZE) {
			rc = filler(env, lp, min_t(size_t, nob, LU_PAGE_SIZE),
				    iops, it, rdpg->rp_attrs, arg);
			if (rc < 0)
				break;
			/* one more lu_page */
			nlupgs++;
			if (rc > 0)
				/* end of index */
				break;
		}
		kunmap(rdpg->rp_pages[i]);
	}

注意filler即为nodemap_page_build
static int nodemap_page_build(const struct lu_env *env, union lu_page *lp,
			      size_t nob, const struct dt_it_ops *iops,
			      struct dt_it *it, __u32 attr, void *arg)
```
上面可以看到，pageidx层，用的是lu_page，这是4K固定的。

lu_rdpg为mdt fill，其代码位置为：mdt/mdt_handler.c

```      
/** input params, should be filled out by mdt */
struct lu_rdpg {
        /** hash */
        __u64                   rp_hash;
        /** count in bytes */
        unsigned int            rp_count;
        /** number of pages */
        unsigned int            rp_npages;
        /** requested attr */
        __u32                   rp_attrs;
        /** pointers to pages */
        struct page           **rp_pages;
};

static int mdt_readpage(struct tgt_session_info *tsi){

......
//rp_count为请求的bytes
	rdpg->rp_count  = min_t(unsigned int, reqbody->mbo_nlink,
				exp_max_brw_size(tsi->tsi_exp));
// rp_npages，可以看到是根据mdt所在的节点的PAGE_SIZE和PAGE_SHIFT计算页数，因此如果是64K page，这里对应的也是物理页。
	rdpg->rp_npages = (rdpg->rp_count + PAGE_SIZE - 1) >>
			  PAGE_SHIFT;
// 分配好对应的page
	OBD_ALLOC_PTR_ARRAY_LARGE(rdpg->rp_pages, rdpg->rp_npages);
	if (rdpg->rp_pages == NULL)
		RETURN(-ENOMEM);

	for (i = 0; i < rdpg->rp_npages; ++i) {
		rdpg->rp_pages[i] = alloc_page(GFP_NOFS);
		if (rdpg->rp_pages[i] == NULL)
			GOTO(free_rdpg, rc = -ENOMEM);
	}

	/* call lower layers to fill allocated pages with directory data */
	rc = mo_readpage(tsi->tsi_env, mdt_object_child(object), rdpg);
	if (rc < 0)
		GOTO(free_rdpg, rc);

	/* send pages to client */
	rc = tgt_sendpage(tsi, rdpg, rc);

	EXIT;

}

 ```