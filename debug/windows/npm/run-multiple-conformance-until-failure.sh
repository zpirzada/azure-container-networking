############################################################################################################
# This script will run multiple rounds of cyclonus until failure. On failure, it will:
# - Capture HNS/VFP state, cluster info, and NPM logs.
# - Stop VMs on the windows nodepool (to stop an HNS trace).

# Requirements:
# - AKS cluster with Windows NPM and Windows Server '22 nodepool named like $WINDOWS_NODEPOOL.
# - Conformance binary with path specified in $E2E_FILE (installation instructions directly below).

# Steps:
# - Identify test cases to run (modifying $toRun and $toSkip).
# - To get the same order every time, set random $SEED (see top of the failed conformance run).
#     For random order, comment out the line referencing $SEED.
# - Create cluster as described above.
# - Create Bastion with defaults on a Windows VMSS instance.
# - Login to Bastion on every node (should have minimum nodes necessary for repro).
# - On each node, run C:\k\starthnstrace.ps1 -MaxFileSize 2000.
# - Start this script with its arguments (see directly below).
############################################################################################################

## Conformance Installation
# git clone https://github.com/huntergregory/kubernetes.git --depth=1 --branch=quit-on-failure
# cd kubernetes
# make WHAT=test/e2e/e2e.test
# cd ../
# mv kubernetes/_output/local/bin/linux/amd64/e2e.test $E2E_FILE

# constants
START=1
END=10
WINDOWS_NODEPOOL=akswin22
E2E_FILE=./e2e-quit-on-failure.test

# IMPORTANT for reproducibility. Find from top of conformance run
# or comment out if randomness is desired
SEED=1672942821

# toRun="NetworkPolicy"

toRun1="Netpol NetworkPolicy between server and client should support allow-all policy"
toRun2="Netpol NetworkPolicy between server and client should allow ingress access from updated pod"
toRun3="Netpol NetworkPolicy between server and client should deny ingress access to updated pod"
toRun4="Netpol NetworkPolicy between server and client should enforce policy to allow traffic based on NamespaceSelector with MatchLabels using default ns label"
toRun5="Netpol NetworkPolicy between server and client should enforce policy to allow traffic only from a pod in a different namespace based on PodSelector and NamespaceSelector"
toRun6="Netpol NetworkPolicy between server and client should enforce policy to allow ingress traffic from pods in all namespaces"
toRun7="Netpol NetworkPolicy between server and client should deny ingress from pods on other namespaces"
toRun8="NetworkPolicy API should support creating NetworkPolicy API operations"
toRun9="Netpol NetworkPolicy between server and client should enforce policy to allow traffic only from a different namespace, based on NamespaceSelector"
toRun="$toRun1|$toRun2|$toRun3|$toRun4|$toRun5|$toRun6|$toRun7|$toRun8|$toRun9"
# 1 hour 5 minutes (10:20:22 to 11:25:33)

nomatch1="should enforce policy based on PodSelector or NamespaceSelector"
nomatch2="should enforce policy based on NamespaceSelector with MatchExpressions using default ns label"
nomatch3="should enforce policy based on PodSelector and NamespaceSelector"
nomatch4="should enforce policy based on Multiple PodSelectors and NamespaceSelectors"
cidrExcept1="should ensure an IP overlapping both IPBlock.CIDR and IPBlock.Except is allowed"
cidrExcept2="should enforce except clause while egress access to server in CIDR block"
namedPorts="named port"
wrongK8sVersion="Netpol API"
toSkip="\[LinuxOnly\]|$nomatch1|$nomatch2|$nomatch3|$nomatch4|$cidrExcept1|$cidrExcept2|$namedPorts|$wrongK8sVersion|SCTP"

# parameters
absoluteKubeConfig=$1
clusterName=$2
resourceGroup=$3

if [[ -z $1 || -z $2 || -z $3 ]]; then
    echo "need absolute path of kubeconfig, and cluster name, and resource group"
    exit 1
fi

aksRGPrefix=MC_$resourceGroup_$clusterName
aksRG=`az group list -otable | grep $aksRGPrefix | awk '{print $1}'`
if [[ -z $aksRG ]]; then
    echo "AKS resource group not found. Should start with $aksRGPrefix..."
    # exit 1
fi
echo "found AKS resource group: $aksRG"
echo "this AKS RG MUST have one WS22 nodepool named $WINDOWS_NODEPOOL..."
echo "START the HNS trace BEFORE running this:  .\starthnstrace.ps1 -maxFileSize 2000 ..."
sleep 15s

echo "beginning conf with kubeconfig: $absoluteKubeConfig, clusterName: $clusterName, resourceGroup: $resourceGroup"

# script
# NOTE: number of folders here impacts cdBack
dateString=`date -I` # like 2022-09-24
base=results-$clusterName/$dateString
mkdir -p $base

set -e
FQDN=`az aks show -n $resourceGroup -g $clusterName --query fqdn -o tsv`
set +e

for i in $(seq $START $END); do
    # delete any old conf-namespaces
    kubectl --kubeconfig $absoluteKubeConfig delete ns -l pod-security.kubernetes.io/enforce | grep "No resources found" || echo "sleeping 3m while HNS state resets" && sleep 3m

    # clear NPM logs and reset HNS state
    echo "restarting npm windows then sleeping 3m"
    kubectl --kubeconfig $absoluteKubeConfig rollout restart -n kube-system ds azure-npm-win
    sleep 3m

    if [[ $i -lt 10 ]]; then
        i="0$i"
    fi
    fname=$base/conf-$i.out
    test -f $fname && echo "conf $i already exists. exiting..." && exit 2
    
    echo "RUNNING CONF #$i at $(date)"
    KUBERNETES_SERVICE_HOST="$FQDN" KUBERNETES_SERVICE_PORT=443 $E2E_FILE \
        --provider=local \
        --node-os-distro=windows \
        --allowed-not-ready-nodes=1 \
        --ginkgo.focus="$toRun" \
        --ginkgo.skip="$toSkip" \
        --kubeconfig=$absoluteKubeConfig \
        --ginkgo.seed=$SEED \
        --delete-namespace=true \
        --delete-namespace-on-failure=true | tee $fname

    cat $fname | grep '"failed":1'
    if [[ $? == 0 ]]; then
        echo "FOUND FAILURE FOR $fname"
        hnsBase=$base/conf-$i-failure-hns-logs
        cdBack=../../..
        mkdir -p $hnsBase
        cd $hnsBase
        ./win-debug.sh $absoluteKubeConfig
        cd $cdBack

        # NPM logs
        for pod in `kubectl --kubeconfig $absoluteKubeConfig get pod -n kube-system | grep azure-npm-win | awk '{print $1}'`; do
            # using -l k8s-app=azure-npm weirdly only gets ~20 lines of log
            kubectl --kubeconfig $absoluteKubeConfig logs -n kube-system $pod > $fname.$pod.log
        fi

        echo "stopping vmss instance to stop hns log capture"
        az vmss stop --instance-ids="*" -n $WINDOWS_NODEPOOL -g $aksRG

        exit 3
    fi

    echo "FINISHED SUCCESSFULLY FOR $fname"
done
