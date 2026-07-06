#include <windows.h>
#include <winsock2.h>
#include <mswsock.h>
#include <stdio.h>

int main() {
    printf("Size: %zu\n", sizeof(RIO_NOTIFICATION_COMPLETION));
    printf("Offset Type: %zu\n", offsetof(RIO_NOTIFICATION_COMPLETION, Type));
    printf("Offset IocpHandle: %zu\n", offsetof(RIO_NOTIFICATION_COMPLETION, Iocp.IocpHandle));
    printf("Offset CompletionKey: %zu\n", offsetof(RIO_NOTIFICATION_COMPLETION, Iocp.CompletionKey));
    printf("Offset Overlapped: %zu\n", offsetof(RIO_NOTIFICATION_COMPLETION, Iocp.Overlapped));
    return 0;
}
