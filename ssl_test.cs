using System;
using System.Net;
using System.Net.Sockets;
using System.Net.Security;
using System.Security.Cryptography.X509Certificates;
using System.Security.Authentication;

public class TestServer {
    public static void Main() {
        var store = new X509Store(StoreName.My, StoreLocation.CurrentUser);
        store.Open(OpenFlags.ReadOnly);
        var certs = store.Certificates.Find(X509FindType.FindBySubjectName, "127.0.0.1", false);
        if (certs.Count == 0) { Console.WriteLine("No cert"); return; }
        
        var listener = new TcpListener(IPAddress.Any, 8086);
        listener.Start();
        Console.WriteLine("Ready");
        
        var client = listener.AcceptTcpClient();
        var ssl = new SslStream(client.GetStream(), false);
        try {
            ssl.AuthenticateAsServer(certs[0], false, SslProtocols.Tls12, false);
            Console.WriteLine("Success! " + ssl.SslProtocol + " " + ssl.CipherAlgorithm + " " + ssl.CipherStrength);
        } catch (Exception ex) {
            Console.WriteLine("FAIL: " + ex.Message);
            if (ex.InnerException != null) Console.WriteLine("INNER: " + ex.InnerException.Message);
        }
    }
}
