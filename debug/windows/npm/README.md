# Introduction 

These scripts will collect Windows NPM logs and the HNS and VFP state of the cluster and write them to a new local folder.

## How to collect logs
In a PowerShell terminal, navigate to the `azure-container-networking/debug/windows/npm folder`. Make sure your kubectl is configured to point to the cluster you want to collect logs from (`az aks get-credentials -g <resource-group> -n <cluster-name>`)
### Windows
Run `.\win-debug.ps1`. The script will create a new folder called logs_DATE containing the results.

### Linux
Run `./win-debug.sh`. The script will create a new folder called logs_DATE containing the results.

Note: You may not be able to unzip logs.zip in Linux since it was compressed in Windows.

## Automating Investigation of Conformance and Cyclonus Failures
These scripts help reproduce even random issues, and on failure, they run the above `./win-debug.sh` and also stop the node to assist with HNS trace capture.

See both `./run-multiple-conformance-until-failure.sh` and `./run-multiple-cyclonus-until-failure.sh` for more info.
