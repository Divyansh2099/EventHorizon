$code = @"
using System;
using System.Runtime.InteropServices;
[StructLayout(LayoutKind.Sequential)]
public struct SCH_CREDENTIALS {
    public uint dwVersion;
    public uint dwWeight;
    public uint cCreds;
    public IntPtr paCred;
    public IntPtr hRootStore;
    public uint cMappers;
    public IntPtr aphMappers;
    public uint cSupportedAlgs;
    public IntPtr palgSupportedAlgs;
    public uint grbitEnabledProtocols;
    public uint dwMinimumCipherStrength;
    public uint dwMaximumCipherStrength;
    public uint dwSessionLifespan;
    public uint dwFlags;
    public uint dwCredFormat;
}
public class Test {
    public static void Run() {
        Console.WriteLine("Size: " + Marshal.SizeOf(typeof(SCH_CREDENTIALS)));
        Console.WriteLine("Offset paCred: " + Marshal.OffsetOf(typeof(SCH_CREDENTIALS), "paCred"));
    }
}
"@
Add-Type -TypeDefinition $code
[Test]::Run()
