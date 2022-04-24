//
// Created by kevin on 2022/2/26.
//

#include <unistd.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <fcntl.h>
#include <linux/fb.h>
#include <sys/mman.h>
#include <sys/ioctl.h>
#include <sys/fcntl.h>
#include <stdarg.h>
#include <sys/stat.h>

static int trace_fd = -1;

void trace_write(const char *fmt, ...)
{
    va_list ap;
    char buf[256];
    int n;

    if (trace_fd < 0)
        return;

    va_start(ap, fmt);
    n = vsnprintf(buf, 256, fmt, ap);
    va_end(ap);

    write(trace_fd, buf, n);
}


int main(int argc , char *argv[])
{
    int i;
    unsigned char *p_map;
    size_t mmap_len = 0;
    unsigned char *mmap_ptr = NULL, junk = 1;
    int fd = -1;
    char *fname;
    struct stat st;
    trace_fd = open("/sys/kernel/debug/tracing/trace_marker", O_WRONLY);
    memset(&st, 0, sizeof(st));
    trace_write("===Multiop start===\n");

    fname = argv[1];

    trace_write("===Multiop O===\n");
    fd = open(fname, O_CREAT | O_RDWR, 0644);
    if (fd == -1) {
        printf("open fail\n");
        exit(1);
    }

    trace_write("===Multiop S===\n");
    if (fstat(fd, &st) == -1) {
        printf("fstat failed\n");
        exit(-1);
    }

    trace_write("===Multiop Before M===\n");
    if (st.st_size == 0) {
        fprintf(stderr,
                "mmap without preceeding stat, or on zero length file.\n");
        exit(-1);
    }
    mmap_len = st.st_size;
    mmap_ptr = mmap(NULL, mmap_len, PROT_WRITE | PROT_READ,
                    MAP_SHARED, fd, 0);
    if (mmap_ptr == MAP_FAILED) {
        exit(-1);
    }
    trace_write("===Multiop W===\n");
    for (i = 0; i < mmap_len && mmap_ptr; i += 4096) {
        trace_write("===Multiop mm_str before===\n");
        mmap_ptr[i] += junk++;
        trace_write("===mmap_str: %d end===\n", i);
    }

    if (munmap(mmap_ptr, mmap_len)) {
        printf("unmap failed\n");
        exit(-1);
    }
    trace_write("===Multiop U===\n");

    if (close(fd) == -1) {
        exit(-1);
    }
    return 0;
}