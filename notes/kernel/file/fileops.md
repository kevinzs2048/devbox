
```c
    case 'O':
        fd = open(fname, O_CREAT | O_RDWR, 0644);
        if (fd == -1) {
            save_errno = errno;
            perror("open(O_RDWR|O_CREAT)");
            exit(save_errno);
        }
        rc = fd;
    case 'S':
        if (fstat(fd, &st) == -1) {
            save_errno = errno;
            perror("fstat");
            exit(save_errno);
        }
        break;
    case 'M':
        if (st.st_size == 0) {
            fprintf(stderr,
                "mmap without preceeding stat, or on zero length file.\n");
            exit(-1);
        }
        mmap_len = st.st_size;
        mmap_ptr = mmap(NULL, mmap_len, PROT_WRITE | PROT_READ,
                MAP_SHARED, fd, 0);
        if (mmap_ptr == MAP_FAILED) {
            save_errno = errno;
            perror("mmap");
            exit(save_errno);
        }
        break;
    case 'W':
        for (i = 0; i < mmap_len && mmap_ptr; i += 4096)
            mmap_ptr[i] += junk++;
        break;
```
