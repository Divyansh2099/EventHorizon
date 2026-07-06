$listener = [System.Net.Sockets.TcpListener]::new(8083)
$listener.Start()
Write-Host "Listening on 8083"
$client = $listener.AcceptTcpClient()
Write-Host "Client connected"
$stream = $client.GetStream()
$sslStream = [System.Net.Security.SslStream]::new($stream, $false)
$cert = Get-ChildItem -Path Cert:\CurrentUser\My | Where-Object Subject -match "127.0.0.1" | Select-Object -First 1
Write-Host "Using cert: $($cert.Subject)"
$sslStream.AuthenticateAsServer($cert)
Write-Host "Authenticated!"
$client.Close()
$listener.Stop()
