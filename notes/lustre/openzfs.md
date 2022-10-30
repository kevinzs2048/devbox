# OpenZFS



## Usage

```
sudo /home/kevin/zfs/zpool create vol0 /dev/loop4
sudo /home/kevin/zfs/zpool add vol0 /dev/loop7 /dev/loop8
```

```


#ifdef __KERNEL__
some code
some code
#endif 

http://linux.laoqinren.net/kernel/how-to-user-__kernel__/
```

ZFS ABD: ARC buffer data (ABD)

## Fletcher4 算法

Fletcher-4 produces a 256-bit result consisting of 4 64-bit numbers. The two Fletcher options are based on the well-known Fletcher Checksum, but differ in the blocksize, checksum length, and checksum number. 

module/zcommon/zfs_fletcher.c

```
fletcher_4_impls定义了一系列fletcher方法，根据CPU的扩展能力计算。
#if defined(__aarch64__) && !defined(__FreeBSD__)
	&fletcher_4_aarch64_neon_ops,
#endif

```

zfs_fletcher_aarch64_neon.c中：

```
const fletcher_4_ops_t fletcher_4_aarch64_neon_ops = {
	.init_native = fletcher_4_aarch64_neon_init,
	.compute_native = fletcher_4_aarch64_neon_native,
	.fini_native = fletcher_4_aarch64_neon_fini,
	.init_byteswap = fletcher_4_aarch64_neon_init,
	.compute_byteswap = fletcher_4_aarch64_neon_byteswap,
	.fini_byteswap = fletcher_4_aarch64_neon_fini,
	.valid = fletcher_4_aarch64_neon_valid,
	.name = "aarch64_neon"
};

对应实现方法的接口：
typedef struct fletcher_4_func {
	fletcher_4_init_f init_native;
	fletcher_4_fini_f fini_native;
	fletcher_4_compute_f compute_native;
	fletcher_4_init_f init_byteswap;
	fletcher_4_fini_f fini_byteswap;
	fletcher_4_compute_f compute_byteswap;
	boolean_t (*valid)(void);
	const char *name;
} fletcher_4_ops_t;

```

几个重要的结构体：

```
typedef struct zfs_fletcher_aarch64_neon {
	uint64_t v[2] __attribute__((aligned(16)));
} zfs_fletcher_aarch64_neon_t;

typedef union fletcher_4_ctx {
	zio_cksum_t scalar;
	zfs_fletcher_superscalar_t superscalar[4];
#if defined(__aarch64__)
	zfs_fletcher_aarch64_neon_t aarch64_neon[4];
#endif
} fletcher_4_ctx_t;

#if defined(__aarch64__)
_ZFS_FLETCHER_H const fletcher_4_ops_t fletcher_4_aarch64_neon_ops;
#endif

```

[Register在C语言中的左右](https://blog.csdn.net/LSZ520LSZ/article/details/121273959)

如果一个变量用register来修饰，则意味着该变量会作为一个[寄存器](https://so.csdn.net/so/search?q=寄存器&spm=1001.2101.3001.7020)变量, 指一个变量直接引用寄存器，也就是对变量名的操作的结果是直接对寄存器进行访问.



```
#define	NEON_MAIN_LOOP()				\
	asm("ld1 { %[SRC].4s }, %[IP]\n"		\ 从IP这个地址加载数据到SRC.4s
	"zip1 %[TMP1].4s, %[SRC].4s, %[ZERO].4s\n"	\ ZIP见下面
	"zip2 %[TMP2].4s, %[SRC].4s, %[ZERO].4s\n"	\
	"add %[ACC0].2d, %[ACC0].2d, %[TMP1].2d\n"	\ ACC0+=TMP1   ACC0_ORI = ACC0INPUT + TMP1
	"add %[ACC1].2d, %[ACC1].2d, %[ACC0].2d\n"	\ ACC1+=ACC0   ACC1_ORI = ACC1INPUT + ACC0_ORI = ACC1INPUT + ACC0INPUT + TMP1
	"add %[ACC2].2d, %[ACC2].2d, %[ACC1].2d\n"	\ ACC2+=ACC1   ACC2_ORI = ACC2INPUT + ACC1_ORI = ACC2INPUT + ACC1INPUT + ACC0INPUT + TMP1
	"add %[ACC3].2d, %[ACC3].2d, %[ACC2].2d\n"	\ ACC3+=ACC2   ACC3_ORI = ACC3INPUT + ACC2_ORI = ACC3INPUT + ACC2INPUT + ACC1INPUT + ACC0INPUT + TMP1
	"add %[ACC0].2d, %[ACC0].2d, %[TMP2].2d\n"	\ ACC0+=TMP2   ACC0 = ACC0_ORI + TMP2 = ACC0INPUT + TMP1 + TMP2
	"add %[ACC1].2d, %[ACC1].2d, %[ACC0].2d\n"	\ ACC1+=ACC0   ACC1 = ACC1_ORI + ACC0 = ACC1INPUT + ACC0INPUT + TMP1 + ACC0INPUT + TMP1 + TMP2
	"add %[ACC2].2d, %[ACC2].2d, %[ACC1].2d\n"	\ ACC2+=ACC1   ACC2 = ACC2_ORI + ACC1 = ACC2INPUT + ACC1INPUT + ACC0INPUT + TMP1 + ACC1INPUT + ACC0INPUT + TMP1 + ACC0INPUT + TMP1 + TMP2
	"add %[ACC3].2d, %[ACC3].2d, %[ACC2].2d\n"	\ ACC3+=ACC2   ACC3 = ACC3_ORI + ACC2 = ACC3INPUT + ACC2_ORI = ACC3INPUT + ACC2INPUT + ACC1INPUT + ACC0INPUT + TMP1 + ACC2INPUT + ACC1INPUT + ACC0INPUT + TMP1 + ACC1INPUT + ACC0INPUT + TMP1 + ACC0INPUT + TMP1 + TMP2
	: [SRC] "=&w" (SRC),				\
	[TMP1] "=&w" (TMP1), [TMP2] "=&w" (TMP2),	\
	[ACC0] "+w" (ACC0), [ACC1] "+w" (ACC1),		\
	[ACC2] "+w" (ACC2), [ACC3] "+w" (ACC3)		\
	: [ZERO] "w" (ZERO), [IP] "Q" (*ip))
```

![ZIP1 and ZIP2 with the arrangement specifier 8B](/Users/kevin/repo/devbox/notes/lustre/zip-asm.png)

ZIP1 <Vd>.<T>, <Vn>.<T>, <Vm>.<T>



### Aarch64 内联汇编基础

Aarch64 内联汇编：中文：http://blog.chinaunix.net/uid-20543672-id-3194385.html

英文：http://www.ethernut.de/en/documents/arm-inline-asm.html

内嵌汇编模版是这样的：

```
asm(code : output operand list : input operand list : clobber list);
```

汇编和C语句这间的联系是通过上面asm声明中可选的output operand list和input operand list。Clobber list后面再讲。



```
/* Rotating bits example */
asm("mov %[result], %[value], ror #1" : [result] "=r" (y) : [value] "r" (x));
```

汇编指令后，是output operand list, 每一个条目是由一对[]（方括号）和被他包括的符号名组成，它后面跟着限制性字符串(operand constraints)，再后面是圆括号和它括着的C变量, 可以有多个条目。

接着冒号后面是input operand list，它的语法和输入操作列表一样



Constraint限制性字符：对于Constraint，GCCArm64的constraint的列表为：

https://gcc.gnu.org/onlinedocs/gcc/Machine-Constraints.html

w：Floating point register, Advanced SIMD vector register or SVE vector register

| **Modifier** | **Specifies**                                            |
| ------------ | -------------------------------------------------------- |
| =            | Write-only operand, usually used for all output operands |
| +            | Read-write operand, must be listed as an output operand  |
| &            | A register that should be used for output only           |



输入输出操作数

Output operands必须为write-only，相应C表达式的值必须是左值

Input operands必须为read-only。比较严格的规则是：不要试图向input operand写。但是如果你想要使用相同的operand作为input和output。限制性modifier（+）可以达到效果。下面的汇编指令，value作为

```
asm("mov %[value], %[value], ror #1" : [value] "+r" (y): [value] "r" (x) )
```

It rotates the contents of the variable *value* to the right by one bit. In opposite to the previous example, the result is not stored in another variable. Instead the original contents of input variable will be modified.





```
#define	NEON_MAIN_LOOP()				\
	asm("add %[ACC0].2d, %[ACC0].2d, %[TMP1].2d\n"	\
	"add %[ACC1].2d, %[ACC1].2d, %[ACC0].2d\n"	\
	"add %[ACC2].2d, %[ACC2].2d, %[ACC1].2d\n"	\
	"add %[ACC3].2d, %[ACC3].2d, %[ACC2].2d\n"	\
	"add %[ACC0].2d, %[ACC0].2d, %[TMP2].2d\n"	\
	"add %[ACC1].2d, %[ACC1].2d, %[ACC0].2d\n"	\
	"add %[ACC2].2d, %[ACC2].2d, %[ACC1].2d\n"	\
	"add %[ACC3].2d, %[ACC3].2d, %[ACC2].2d\n"	\
	: [SRC] "=&w" (SRC),				\
	[ACC0] "+w" (ACC0), [ACC1] "+w" (ACC1),		\
	[ACC2] "+w" (ACC2), [ACC3] "+w" (ACC3)		\
	:[TMP1] "w" (TMP1), [TMP2] "w" (TMP2))

#define	NEON_MAIN_LOOP_LOAD(REVERSE)			\
	asm("ld1 { %[SRC].4s }, %[IP]\n"		\
	REVERSE						\
    "zip1 %[TMP1].4s, %[SRC].4s, %[ZERO].4s\n"	\
	"zip2 %[TMP2].4s, %[SRC].4s, %[ZERO].4s\n"	\
    : [SRC] "=&w" (SRC),[TMP1] "=&w" (TMP1), [TMP2] "=&w" (TMP2)	\
    : [ZERO] "w" (ZERO), [IP] "Q" (*ip))

```



```
0000000000000168 <fletcher_4_aarch64_neon_native>:
 168:	a9bd7bfd 	stp	x29, x30, [sp, #-48]!
 16c:	910003fd 	mov	x29, sp
 170:	a90153f3 	stp	x19, x20, [sp, #16]
 174:	a9025bf5 	stp	x21, x22, [sp, #32]
 178:	d50320ff 	xpaclri
 17c:	aa0003f6 	mov	x22, x0
 180:	aa1e03e0 	mov	x0, x30
 184:	aa0103f3 	mov	x19, x1
 188:	8b020034 	add	x20, x1, x2
 18c:	aa0203f5 	mov	x21, x2
 190:	94000000 	bl	0 <_mcount>
 194:	94000000 	bl	0 <kernel_neon_begin>
 198:	eb14027f 	cmp	x19, x20
 19c:	540002c2 	b.cs	1f4 <fletcher_4_aarch64_neon_native+0x8c>  // b.hs, b.nlast
 1a0:	91004273 	add	x19, x19, #0x10
 1a4:	eb13029f 	cmp	x20, x19
 1a8:	54ffffc8 	b.hi	1a0 <fletcher_4_aarch64_neon_native+0x38>  // b.pmore
 1ac:	d10006a1 	sub	x1, x21, #0x1
 1b0:	d344fc21 	lsr	x1, x1, #4
 1b4:	11000421 	add	w1, w1, #0x1
 1b8:	90000000 	adrp	x0, 0 <fletcher_4_aarch64_neon_fini>
 1bc:	91000000 	add	x0, x0, #0x0
 1c0:	94000000 	bl	0 <printk>
 1c4:	910042c0 	add	x0, x22, #0x10
 1c8:	910082c1 	add	x1, x22, #0x20
 1cc:	9100c2c2 	add	x2, x22, #0x30
 1d0:	4c007ac1 	st1	{v1.4s}, [x22]
 1d4:	4c007802 	st1	{v2.4s}, [x0]
 1d8:	4c007823 	st1	{v3.4s}, [x1]
 1dc:	4c007844 	st1	{v4.4s}, [x2]
 1e0:	94000000 	bl	0 <kernel_neon_end>
 1e4:	a94153f3 	ldp	x19, x20, [sp, #16]
 1e8:	a9425bf5 	ldp	x21, x22, [sp, #32]
 1ec:	a8c37bfd 	ldp	x29, x30, [sp], #48
 1f0:	d65f03c0 	ret
 1f4:	52800001 	mov	w1, #0x0                   	// #0
 1f8:	17fffff0 	b	1b8 <fletcher_4_aarch64_neon_native+0x50>
 1fc:	d503201f 	nop
 
 
 
 0000000000000168 <fletcher_4_aarch64_neon_native>:
 168:	a9bd7bfd 	stp	x29, x30, [sp, #-48]!
 16c:	910003fd 	mov	x29, sp
 170:	a90153f3 	stp	x19, x20, [sp, #16]
 174:	f90013f5 	str	x21, [sp, #32]
 178:	aa0003f4 	mov	x20, x0
 17c:	d50320ff 	xpaclri
 180:	aa1e03e0 	mov	x0, x30
 184:	8b020035 	add	x21, x1, x2
 188:	aa0103f3 	mov	x19, x1
 18c:	94000000 	bl	0 <_mcount>
 190:	94000000 	bl	0 <kernel_neon_begin>
 194:	91004280 	add	x0, x20, #0x10
 198:	91008281 	add	x1, x20, #0x20
 19c:	9100c282 	add	x2, x20, #0x30
 1a0:	eb15027f 	cmp	x19, x21
 1a4:	6e201c00 	eor	v0.16b, v0.16b, v0.16b
 1a8:	4c407a81 	ld1	{v1.4s}, [x20]
 1ac:	4c407802 	ld1	{v2.4s}, [x0]
 1b0:	4c407823 	ld1	{v3.4s}, [x1]
 1b4:	4c407844 	ld1	{v4.4s}, [x2]
 1b8:	54000202 	b.cs	1f8 <fletcher_4_aarch64_neon_native+0x90>  // b.hs, b.nlast
 1bc:	d503201f 	nop
 1c0:	4c407a67 	ld1	{v7.4s}, [x19]
 1c4:	4e8038e5 	zip1	v5.4s, v7.4s, v0.4s
 1c8:	4e8078e6 	zip2	v6.4s, v7.4s, v0.4s
 1cc:	4ee58421 	add	v1.2d, v1.2d, v5.2d
 1d0:	4ee18442 	add	v2.2d, v2.2d, v1.2d
 1d4:	4ee28463 	add	v3.2d, v3.2d, v2.2d
 1d8:	4ee38484 	add	v4.2d, v4.2d, v3.2d
 1dc:	4ee68421 	add	v1.2d, v1.2d, v6.2d
 1e0:	4ee18442 	add	v2.2d, v2.2d, v1.2d
 1e4:	4ee28463 	add	v3.2d, v3.2d, v2.2d
 1e8:	4ee38484 	add	v4.2d, v4.2d, v3.2d
 1ec:	91004273 	add	x19, x19, #0x10
 1f0:	eb1302bf 	cmp	x21, x19
 1f4:	54fffe68 	b.hi	1c0 <fletcher_4_aarch64_neon_native+0x58>  // b.pmore
 1f8:	91004280 	add	x0, x20, #0x10
 1fc:	91008281 	add	x1, x20, #0x20
 200:	9100c282 	add	x2, x20, #0x30
 204:	4c007a81 	st1	{v1.4s}, [x20]
 208:	4c007802 	st1	{v2.4s}, [x0]
 20c:	4c007823 	st1	{v3.4s}, [x1]
 210:	4c007844 	st1	{v4.4s}, [x2]
 214:	94000000 	bl	0 <kernel_neon_end>
 218:	a94153f3 	ldp	x19, x20, [sp, #16]
 21c:	f94013f5 	ldr	x21, [sp, #32]
 220:	a8c37bfd 	ldp	x29, x30, [sp], #48
 224:	d65f03c0 	ret
```

