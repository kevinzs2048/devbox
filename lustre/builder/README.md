# The Lustre Builder

This repo include all the script we needs for Lustre Arm64 external Builder and Tester CI

## Lustre CI pipeline

```mermaid
graph pipeline;
A[Terraform OpenStack]-->B[Node Init];
B[Node Init]-->C[Test Cluster Reboot];
C[Test Cluster Reboot]-->D[Auster Run Test];
D[Auster Run Test]-->E[Upload Maloo DB];
E[Upload Maloo DB]-->F[Keep/Destroy Cluster];
```
