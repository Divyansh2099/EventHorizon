$cert = New-SelfSignedCertificate -DnsName '127.0.0.1' -CertStoreLocation 'Cert:\CurrentUser\My' -KeyAlgorithm RSA -KeyLength 2048 -KeyExportPolicy Exportable
$pwd = ConvertTo-SecureString -String 'password' -Force -AsPlainText
Export-PfxCertificate -Cert $cert -FilePath 'cng_cert.pfx' -Password $pwd
