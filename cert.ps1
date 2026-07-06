$cert = New-SelfSignedCertificate -DnsName '127.0.0.1' -CertStoreLocation 'Cert:\CurrentUser\My' -Provider 'Microsoft RSA SChannel Cryptographic Provider' -KeySpec KeyExchange -KeyAlgorithm RSA -KeyLength 2048
$pwd = ConvertTo-SecureString -String 'password' -Force -AsPlainText
Export-PfxCertificate -Cert $cert -FilePath 'windows_cert.pfx' -Password $pwd
