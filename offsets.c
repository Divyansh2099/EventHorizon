#include <windows.h>
#include <schannel.h>
#include <stdio.h>
#include <stddef.h>

int main() {
    printf("Size: %zu\n", sizeof(SCH_CREDENTIALS));
    printf("dwVersion: %zu\n", offsetof(SCH_CREDENTIALS, dwVersion));
    printf("dwCredFormat: %zu\n", offsetof(SCH_CREDENTIALS, dwCredFormat));
    printf("cCreds: %zu\n", offsetof(SCH_CREDENTIALS, cCreds));
    printf("paCred: %zu\n", offsetof(SCH_CREDENTIALS, paCred));
    printf("hRootStore: %zu\n", offsetof(SCH_CREDENTIALS, hRootStore));
    printf("cMappers: %zu\n", offsetof(SCH_CREDENTIALS, cMappers));
    printf("aphMappers: %zu\n", offsetof(SCH_CREDENTIALS, aphMappers));
    printf("cSupportedAlgs: %zu\n", offsetof(SCH_CREDENTIALS, cSupportedAlgs));
    printf("palgSupportedAlgs: %zu\n", offsetof(SCH_CREDENTIALS, palgSupportedAlgs));
    printf("grbitEnabledProtocols: %zu\n", offsetof(SCH_CREDENTIALS, grbitEnabledProtocols));
    printf("dwMinimumCipherStrength: %zu\n", offsetof(SCH_CREDENTIALS, dwMinimumCipherStrength));
    printf("dwMaximumCipherStrength: %zu\n", offsetof(SCH_CREDENTIALS, dwMaximumCipherStrength));
    printf("dwSessionLifespan: %zu\n", offsetof(SCH_CREDENTIALS, dwSessionLifespan));
    printf("dwFlags: %zu\n", offsetof(SCH_CREDENTIALS, dwFlags));
    printf("cTlsExts: %zu\n", offsetof(SCH_CREDENTIALS, cTlsExts));
    printf("pTlsExts: %zu\n", offsetof(SCH_CREDENTIALS, pTlsExts));
    return 0;
}
