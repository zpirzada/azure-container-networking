############################################################################################################
# This script will run multiple rounds of cyclonus until failure. On failure, it will:
# - Capture HNS/VFP state, cluster info, and NPM logs.
# - Stop VMs on the windows nodepool (to stop an HNS trace).

# Requirements:
# - AKS cluster with Windows NPM and Windows Server '22 nodepool named like $WINDOWS_NODEPOOL.
# - Cyclonus binary in the same folder as this script, named "cyclonus-stop-on-failure" (installation below).

# Steps:
# - Create cluster as described above.
# - Create Bastion with defaults on a Windows VMSS instance.
# - Login to Bastion on every node (should have minimum nodes necessary for repro).
# - On each node, run C:\k\starthnstrace.ps1 -MaxFileSize 2000.
# - Start this script with its arguments (see directly below).
############################################################################################################

## Cyclonus Installation
# git checkout https://github.com/huntergregory/cyclonus.git --depth=1 --branch=stop-after-failure
# cd cyclonus
# CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./cmd/cyclonus/cyclonus ./cmd/cyclonus
# cd ..
# mv cyclonus/cmd/cyclonus/cyclonus cyclonus-stop-on-failure

# constants
START=1
END=10
WINDOWS_NODEPOOL=akswin22

# parameters
absolutePathKubeConfig=$1
kubecontext=$2
clusterName=$3
resourceGroup=$4

if [[ -z $1 || -z $2 || -z $3 || -z $4 ]]; then
    echo "need absolute path of kubeconfig, and kubecontext string"
    exit 1
fi

aksRGPrefix=MC_$resourceGroup_$clusterName
aksRG=`az group list -otable | grep $aksRGPrefix | awk '{print $1}'`
if [[ -z $aksRG ]]; then
    echo "AKS resource group not found. Should start with $aksRGPrefix..."
    exit 1
fi
echo "found AKS resource group: $aksRG"
echo "this AKS RG MUST have one WS22 nodepool named $WINDOWS_NODEPOOL..."
echo "START the HNS trace BEFORE running this:  .\starthnstrace.ps1 -maxFileSize 2000 ..."
sleep 15s

echo "using kubeconfig: $absolutePathKubeConfig"
echo "using kubecontext: $kubecontext"

dateString=`date -I` # like 2022-09-24

# NOTE: number of folders here impacts cdBack
base=results-$kubecontext/$dateString
mkdir -p results-$kubecontext
test -d $base && echo "folder $base/ already exists" && exit 1
mkdir $base
for i in $(seq $START $END); do
    echo "round $i"
    if [[ i -lt 10 ]]; then
        i=0$i
    fi
    roundBase=$base/round-$i
    mkdir $roundBase || echo "folder $roundBase already exists" && exit 1
    cd $roundBase
    cdBack=../../..

    ## run cyc
    kubectl delete ns x y z --kubeconfig $absolutePathKubeConfig

    # clear NPM logs and reset HNS state
    echo "restarting npm windows then sleeping 3m"
    kubectl --kubeconfig $absoluteKubeConfig rollout restart -n kube-system ds azure-npm-win
    sleep 3m

    set -x
    LOG_FILE=run.out
    $cdBack/cyclonus-stop-on-failure generate \
        --context=$kubecontext \
        --noisy=true \
        --retries=3 \
        --ignore-loopback=true \
        --cleanup-namespaces=true \
        --perturbation-wait-seconds=20 \
        --pod-creation-timeout-seconds=480 \
        --job-timeout-seconds=15 \
        --server-protocol=TCP,UDP \
        --exclude sctp,named-port,ip-block-with-except,multi-peer,upstream-e2e,example,end-port,namespaces-by-default-label,update-policy | tee $LOG_FILE
    set +x

    # might need to redirect to /dev/null 2>&1 instead of just grepping with -q to avoid "cat: write error: Broken pipe"
    rc=999
    cat $LOG_FILE | grep "SummaryTable:" > /dev/null 2>&1 && rc=$?
    echo $rc
    if [ $rc -ne 0 ]; then
        echo "FAILING because cyclonus tests did not complete" | tee -a $LOG_FILE
        exit 2
    fi

    rc=0
    cat $LOG_FILE | grep "failed" > /dev/null 2>&1 || rc=$?
    echo $rc
    if [ $rc -eq 0 ]; then
        echo "FAILING because cyclonus completed but failures detected" | tee -a $LOG_FILE
        ./win-debug.sh $absolutePathKubeConfig | tee -a $LOG_FILE

        # NPM logs
        for pod in `kubectl --kubeconfig $absoluteKubeConfig get pod -n kube-system | grep azure-npm-win | awk '{print $1}'`; do
            # using -l k8s-app=azure-npm weirdly only gets ~20 lines of log
            kubectl --kubeconfig $absoluteKubeConfig logs -n kube-system $pod > $fname.$pod.log
        done

        echo "stopping vmss instance to stop hns log capture"
        az vmss stop --instance-ids="*" -n $WINDOWS_NODEPOOL -g $aksRG

        exit 3
    fi

    echo "FINISHED SUCCESSFULLY FOR ROUND $i"
    cd $cdBack
done
