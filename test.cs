using System;
using System.Runtime.InteropServices;
using System.Security.Cryptography.X509Certificates;

public class TlsTest
{
    [StructLayout(LayoutKind.Sequential)]
    public struct SCH_CREDENTIALS
    {
        public uint dwVersion;
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

    [StructLayout(LayoutKind.Sequential)]
    public struct SecBuffer
    {
        public uint cbBuffer;
        public uint BufferType;
        public IntPtr pvBuffer;
    }

    [StructLayout(LayoutKind.Sequential)]
    public struct SecBufferDesc
    {
        public uint ulVersion;
        public uint cBuffers;
        public IntPtr pBuffers;
    }

    [StructLayout(LayoutKind.Sequential)]
    public struct CredHandle
    {
        public IntPtr dwLower;
        public IntPtr dwUpper;
    }

    [DllImport("secur32.dll", CharSet = CharSet.Unicode)]
    public static extern int AcquireCredentialsHandleW(
        string pszPrincipal,
        string pszPackage,
        uint fCredentialUse,
        IntPtr pvLogonID,
        ref SCH_CREDENTIALS pAuthData,
        IntPtr pGetKeyFn,
        IntPtr pvGetKeyArgument,
        out CredHandle phCredential,
        out long ptsExpiry);

    [DllImport("secur32.dll", CharSet = CharSet.Unicode)]
    public static extern int AcceptSecurityContext(
        ref CredHandle phCredential,
        IntPtr phContext,
        ref SecBufferDesc pInput,
        uint fContextReq,
        uint TargetDataRep,
        out CredHandle phNewContext,
        ref SecBufferDesc pOutput,
        out uint pfContextAttr,
        out long ptsTimeStamp);

    public static unsafe void Main()
    {
        var store = new X509Store(StoreName.My, StoreLocation.CurrentUser);
        store.Open(OpenFlags.ReadOnly);
        var certs = store.Certificates.Find(X509FindType.FindByThumbprint, "EE0495D51ECAF98835BF244EC692B6D10A33795B", false);
        if (certs.Count == 0) return;
        var cert = certs[0];

        IntPtr[] certArray = new IntPtr[1];
        certArray[0] = cert.Handle;

        SCH_CREDENTIALS creds = new SCH_CREDENTIALS();
        creds.dwVersion = 4;
        creds.cCreds = 1;
        creds.grbitEnabledProtocols = 0;
        creds.dwFlags = 0;
        
        fixed (IntPtr* pCerts = certArray)
        {
            creds.paCred = (IntPtr)pCerts;
            
            CredHandle hCreds;
            long tsExpiry;
            int status = AcquireCredentialsHandleW(
                null, "Schannel", 1, IntPtr.Zero, ref creds, IntPtr.Zero, IntPtr.Zero, out hCreds, out tsExpiry);
            Console.WriteLine("Acquire: {0:X}", status);

            var listener = new System.Net.Sockets.TcpListener(System.Net.IPAddress.Any, 8085);
            listener.Start();
            Console.WriteLine("Listening on 8085");
            var client = listener.AcceptTcpClient();
            var stream = client.GetStream();
            byte[] clientHello = new byte[4096];
            int read = stream.Read(clientHello, 0, clientHello.Length);
            Console.WriteLine("Read " + read + " bytes");
            
            byte[] exactHello = new byte[read];
            Array.Copy(clientHello, exactHello, read);

            fixed (byte* pClientHello = exactHello)
            {
                SecBuffer[] inBufs = new SecBuffer[2];
                inBufs[0].cbBuffer = (uint)exactHello.Length;
                inBufs[0].BufferType = 2; // TOKEN
                inBufs[0].pvBuffer = (IntPtr)pClientHello;
                inBufs[1].cbBuffer = 0;
                inBufs[1].BufferType = 0; // EMPTY
                inBufs[1].pvBuffer = IntPtr.Zero;

                SecBufferDesc inDesc = new SecBufferDesc();
                inDesc.ulVersion = 0;
                inDesc.cBuffers = 2;
                fixed (SecBuffer* pInBufs = inBufs)
                {
                    inDesc.pBuffers = (IntPtr)pInBufs;
                    
                    SecBuffer[] outBufs = new SecBuffer[3];
                    outBufs[0].cbBuffer = 0;
                    outBufs[0].BufferType = 2;
                    outBufs[0].pvBuffer = IntPtr.Zero;
                    outBufs[1].cbBuffer = 0;
                    outBufs[1].BufferType = 17; // ALERT
                    outBufs[1].pvBuffer = IntPtr.Zero;
                    outBufs[2].cbBuffer = 0;
                    outBufs[2].BufferType = 0;
                    outBufs[2].pvBuffer = IntPtr.Zero;

                    SecBufferDesc outDesc = new SecBufferDesc();
                    outDesc.ulVersion = 0;
                    outDesc.cBuffers = 3;
                    fixed (SecBuffer* pOutBufs = outBufs)
                    {
                        outDesc.pBuffers = (IntPtr)pOutBufs;

                        CredHandle hCtx;
                        uint fContextAttr;
                        long tsTimeStamp;
                        uint flags = 0x00000010 | 0x00000008 | 0x00000004 | 0x00010000 | 0x00000100 | 0x00008000;
                        
                        status = AcceptSecurityContext(
                            ref hCreds, IntPtr.Zero, ref inDesc, flags, 0,
                            out hCtx, ref outDesc, out fContextAttr, out tsTimeStamp);
                        
                        Console.WriteLine("Accept: {0:X}", status);
                    }
                }
            }
        }
    }
}
