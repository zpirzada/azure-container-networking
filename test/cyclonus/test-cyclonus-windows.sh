curl -fsSL github.com/mattfenwick/cyclonus/releases/latest/download/cyclonus_linux_amd64.tar.gz | tar -zxv
# As of 09/14, TTL for Windows flow table is up to 240 seconds.
# Probes take at least 20 seconds (from logs), so with 7 retries, we will start our last try at least 140 seconds after perturbation.
# Therefore, setting perturbation-wait-seconds=100 to avoid the effects of current TTL.
./cyclonus_linux_amd64/cyclonus generate \
    --noisy=true \
    --retries=7 \
    --ignore-loopback=true \
    --cleanup-namespaces=true \
    --perturbation-wait-seconds=80 \
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
