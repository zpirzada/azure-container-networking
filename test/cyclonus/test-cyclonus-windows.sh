# curl -fsSL github.com/mattfenwick/cyclonus/releases/latest/download/cyclonus_linux_amd64.tar.gz | tar -zxv
# ./cyclonus_linux_amd64/cyclonus generate \
git clone --branch sleep-and-retry-after-failing --depth=1 https://github.com/huntergregory/cyclonus.git
cd cyclonus/
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./cmd/cyclonus/cyclonus ./cmd/cyclonus
./cmd/cyclonus/cyclonus generate \
    --noisy=true \
    --retries=7 \
    --ignore-loopback=true \
    --cleanup-namespaces=true \
    --perturbation-wait-seconds=20 \
    --pod-creation-timeout-seconds=480 \
    --job-timeout-seconds=15 \
    --server-protocol=TCP,UDP \
    --exclude sctp,named-port,ip-block-with-except,multi-peer,upstream-e2e,example,end-port,namespaces-by-default-label,update-policy | tee cyclonus-$CLUSTER_NAME

rc=0
cat cyclonus-$CLUSTER_NAME | grep "failed" > /dev/null 2>&1 || rc=$?
echo $rc
if [ $rc -eq 0 ]; then
    echo "failures detected"
    exit 1
fi
