# EventHorizon 100k+ RPS Hardware Tuning

To break the 100,000 Requests Per Second barrier, the underlying host operating system and network interface must be tuned to allow Receive Side Scaling (RSS). This prevents the host from funneling all network interrupts to CPU Core 0.

## Enable Receive Side Scaling (RSS)

Run the following commands in an Administrator PowerShell prompt:

```powershell
# Enable RSS globally on all network adapters
Enable-NetAdapterRss -Name "*"

# Verify that RSS is enabled and multiple queues are available
Get-NetAdapterRss
```

If you are running in a virtual machine (like Hyper-V), ensure that Virtual Machine Queues (VMQ) or equivalent paravirtualized RSS features are enabled on the host.
