# How to Run EventHorizon Locally

EventHorizon is a zero-allocation, hardware-accelerated web framework for Go. Because it hooks directly into Windows hardware (RIO) and the native Windows Security layer (SSPI), there are a few specific steps required to run it on your local machine.

## Prerequisites
1. **Operating System:** Windows 10, Windows 11, or Windows Server. (Linux/macOS are not supported due to the dependency on Windows Registered I/O).
2. **Go:** Ensure you have Go installed (`v1.20+` recommended).

---

## Step-by-Step Guide

### 1. Clone the Repository
Open your terminal or PowerShell and clone the codebase:
```bash
git clone https://github.com/Divyansh2099/EventHorizon.git
cd EventHorizon
```

### 2. Install the Developer Certificate (Crucial!)
EventHorizon uses native Windows SSPI for extreme TLS speed. Instead of loading certificate files from disk, it securely reads them directly from the **Windows Certificate Store**. 

If you do not have a development certificate in your Windows Store, the server will crash on boot with a `Could not find a valid certificate with a private key` error.

To fix this, **open PowerShell as Administrator** and run the provided script:
```powershell
.\cert.ps1
```

*(If your system restricts running scripts, you can paste this exact command into PowerShell instead):*
```powershell
New-SelfSignedCertificate -DnsName '127.0.0.1' -CertStoreLocation 'Cert:\CurrentUser\My' -Provider 'Microsoft RSA SChannel Cryptographic Provider' -KeySpec KeyExchange -KeyAlgorithm RSA -KeyLength 2048
```

### 3. Run the Showcase Dashboard
Once the certificate is securely in your Windows Store, you can boot the server:
```bash
go run ./cmd/showcase
```

### 4. View the Portfolio
Open your favorite web browser and navigate to the live dashboard:
👉 **https://127.0.0.1:8082**

*Note: Because you are using a self-signed development certificate, your browser will show a "Your connection is not private" or "Potential Security Risk" warning. This is completely normal for local development. Simply click **Advanced** -> **Proceed to 127.0.0.1** to view the site.*

---

## Troubleshooting

### Error: `AcceptSecurityContext failed with status 0x80090327`
If the server boots successfully and says `Found certificate with private key!`, but the moment you try to connect via your browser you see a flood of `AcceptSecurityContext failed` errors in the server console, it means your Windows store has **multiple conflicting `127.0.0.1` certificates**.

EventHorizon will grab the first one it finds, and if it's an old certificate with an inaccessible private key (which often happens after a system restart), the TLS handshake will fail.

**The Fix:**
Open PowerShell as Administrator and run this command to delete all old conflicting certificates and generate a single fresh one:
```powershell
Get-ChildItem -Path Cert:\CurrentUser\My | Where-Object { $_.Subject -match '127.0.0.1' } | Remove-Item -Force
New-SelfSignedCertificate -DnsName '127.0.0.1' -CertStoreLocation 'Cert:\CurrentUser\My' -Provider 'Microsoft RSA SChannel Cryptographic Provider' -KeySpec KeyExchange -KeyAlgorithm RSA -KeyLength 2048
```
Then, restart the server.
