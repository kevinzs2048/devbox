# pKVM

## 背景需要

Armv8有多个特权级别，称为异常级别，从特权最高的固件(EL3)，到hypervisor (EL2)和操作系统(EL1)级别，再到特权最低的用户(EL0)级别。EL2不是固件，设备制造商更新此部分代码不会有太大影响，也不是操作系统级别，不需要与其他东西集成。因此塞进来了很多特性。

Arm长期以来如何拥有并行的“受信任”端，应用程序可以在受信任的操作系统和hypervisor上运行。“受信任”的定义只是“总线上的一个位”，它允许更多地访问物理内存。一定要注意，受信任端上的代码可以访问所有内存，而不受信任端上的代码不能访问仅受信任的内存。信任的级别都比非安全级别享有更多的特权，因此受信任的操作系统可以映射不受信任的管理程序内存，例如，它可以提供访问，以便受信任的应用程序也可以访问它。

`Android世界中是有问题的，部分原因在于通常在可信端运行的内容:用于数字版权管理(DRM)的第三方代码、各种不透明的二进制blob、加密代码等等。这些代码可能不值得信任，而且还存在碎片问题。`

Android系统必须希望在受信任端运行的软件不是恶意的或被破坏的，期望有一种方法去剥夺这些第三方代码的特权。需要一个可移植的环境，以一种与Android系统隔离的方式托管这些服务。该机制还将这些第三方程序彼此隔离。



## 虚拟内存管理



安全VM，基于nVHE。在server上可能会有并存的问题。但是安卓上因为安全考虑，禁止了VHE，因此引入性能还好。但server环境中，这会是个问题。

linux image mapping KVM

pKVM的解决办法

将 trusted 代码移到与 Android 系统相同 level 的虚拟机中。由于目前 Android 系统中还没有虚拟机(VM)，如果有一个 hypervisor 可以管理 VM 的话，我们就可以加一些 VM 进来。利用GKI讲KVM作为hypervisor，降低碎片化。以便将那些第三方代码从权限过高的 trusted 世界中移出来。

在 nVHE 模式下，host 的 kernel 和 guest kernel 都运行在操作系统级别（EL1），而在 EL2 hypervisor 级别有一个虚拟机监视器（VMM, virtual machine monitor）。由于 host kernel 没有直接切换到 guest 所需的权限，因此需要由 VMM 来进行 "world switch"才能实现，这使得 nVHE 模式的运行速度相对来说比较慢（With nVHE, EL2 contains some small "world-switch" code installed by the kernel and it is this layer that we are working to extend for Protected KVM）

VHE模式下，kernel和VMM放入TCM中，host kernel可以访问guest所有的内存空间，是的安卓系统进入了特权模式。Android security model 要求，即使 host kernel 被入侵，guest 的数据也要保持私密性，而使用 VHE 的 KVM 并没有达到这样的效果。采用nVHE模式，我们可以：only the `world-switch` piece needs to be trusted. It can be extended to manage the `stage-2 page tables` and manage other functions for the guests. 



TCB:**https://aliceevebob.com/2019/10/22/whats-a-trusted-compute-base/**



## 拓展阅读

### Hypervisor

Hypervisor运行在EL2异常级别。只有运行在EL2或更高异常级别的软件才可以访问并配置各项虚拟化功能。

- **Stage 2转换**
- **EL1/0指令和寄存器访问**
- 注入虚拟异常

![img](/Users/kevin/repo/devbox/notes/confidential_computing/virtualization-aarch64.png)

安全状态的EL2用灰色显示是因为，安全状态的EL2并不总是可用，这是Armv8.4-A引入的特性。







### VHE

在nVHE情况下，两种虚拟化类型的软件架构

Type1类型，Hypervisor运行在EL2，VM运行在EL0/1

![img](/Users/kevin/repo/devbox/notes/confidential_computing/type1.png)

Type2类型

![img](/Users/kevin/repo/devbox/notes/confidential_computing/type2.png)

Host内核部分运行在EL1，控制虚拟化的部分运行在EL2。明显的缺陷是：VHE之前的hypervisor需要设计成High-visor和low-visor两部分，前者运行在EL1，后者运行在EL2。分层设计在系统运行时造成了不必要的上下文切换。为了解决该问题，引入了虚拟化主机扩展 （Virtualization Host Extensions, VHE). Armv8.1-A引入，可以让Host操作系统的内核部分直接运行在EL2上。



VHE由系统寄存器HCR_EL2中的两个比特位控制

- E2H：VHE使能位
- TGE：当VHE使能时，控制EL0是Guest还是Host

![img](/Users/kevin/repo/devbox/notes/confidential_computing/vhe.png)

### 安全世界虚拟化

Armv8.4-A增加了安全世界下EL2的支持。支持安全世界EL2的处理器，需配置EL3下的SCR_EL3.EEL2比特位来开启这一特性。设置了这一比特位，才允许使用安全状态下的虚拟化功能。

在安全世界虚拟化之前，EL3通常用于运行安全状态切换软件和平台固件。然而从设计上来说，我们希望EL3中运行的软件越少越好，因为越简单才会更安全。安全状态虚拟化使得我们可以将平台固件移到EL1中运行，由虚拟化来隔离平台固件和可信操作系统内核。

![img](/Users/kevin/repo/devbox/notes/confidential_computing/sel2.png)



Arm体系结构定义了安全世界和非安全世界两个物理地址空间。非安全状态下，stage 1转换的输出总是非安全的，因此只需要一个IPA空间来给stage 2使用. 对于安全世界，stage 1的输出可能时安全的也能是非安全的。Stage 1转换表中的NS比特位控制使用安全地址还是非安全地址。这意味着在安全世界，需要两个IPA地址空间。



与stage 1表不同，stage 2转换表中没有NS比特位。因为对于一个特定的IPA空间，要么全都是安全地址，要么全都是非安全的，因此只需要由一个寄存器比特位来确定IPA空间。通常来说，非安全地址经过stage 2转换仍然是非安全地址，安全地址经过stage 2转换仍然是安全地址。



![img](/Users/kevin/repo/devbox/notes/confidential_computing/secure-world.png)



## Reference

https://lwn.net/Articles/836693/

[Arm64虚拟化介绍](https://calinyara.github.io/technology/2019/11/03/armv8-virtualization.html)

[LWN 介绍pKVM的中文翻译](https://mp.weixin.qq.com/s/QKZFqxRPlxljeC7EBaGyGg)

[Android virtualization官方中文文档](https://source.android.com/docs/core/virtualization/architecture)