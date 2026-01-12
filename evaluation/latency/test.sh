#!/bin/bash
# set -euo pipefail

##########################
# CONSTANTS – CONFIGURE THESE AS NEEDED
##########################
SSH_PASSWORD="olaadeus"                     # SSH password for the Kubernetes cluster machines
MACHINE_USER="ubuntu"                     # SSH user (assumed to have sudo privileges for reboot)
EXTERNAL_IP="10.255.30.152"
MACHINES=("10.255.30.152" "10.255.30.196" "10.255.30.244")  # IPs of the 3 Kubernetes machines and the code-gen
WAIT_PERIOD=1                                    # Seconds to wait between each request
NAMESPACES=("sample-app")
WAIT_REBOOT=300                                  # Seconds to wait after rebooting the cluster machines
TESTS=("baseline" "enforce")
NUMBER_TESTS=550
MAX_RULES=500
RULES_JUMP_SIZE=10
ENTRY_POINT="sample-function"
ENFORCER_QUEUE="ghcr.io/atnog/serverless-workflow-firewall/queue:latest"
BASELINE_QUEUE="gcr.io/knative-releases/knative.dev/serving/cmd/queue:v1.19.5"

# Base directory to store test results (trace file and pod logs)
BASE_RESULT_DIR="./results"
mkdir -p "$BASE_RESULT_DIR"

# Wait until all cluster nodes are Ready
wait_for_cluster() {
    echo "Waiting for all cluster nodes to be Ready..."
    while true; do
        not_ready=$(kubectl get nodes | tail -n 3 | grep -v " Ready" | wc -l)
        if [ "$not_ready" -eq 0 ]; then
            echo "All cluster nodes are Ready."
            break
        else
            echo "Some nodes are not Ready. Waiting 10 seconds..."
            sleep 10
        fi
    done
}

# Reboot all machines via SSH (requires sshpass)
reboot_machines() {
    echo "Rebooting cluster machines..."
    for machine in "${MACHINES[@]}"; do
        echo "Rebooting machine $machine..."
        sshpass -p "$SSH_PASSWORD" ssh -o StrictHostKeyChecking=no "$MACHINE_USER@$machine" "sudo reboot" &
    done
    echo "Waiting $WAIT_REBOOT seconds for machines to reboot..."
    sleep "$WAIT_REBOOT"
}

generate_rules() {
  cat <<EOF
request:
  default-action: accept
  rules:
EOF

  for i in $(seq 1 "$1"); do
    cat <<EOF
    - action: reject
      expression: |
        ([.REQUEST.BODY.username?] | map(tostring) | join(" ")) | test("{{|}}|{%-|-%}|__class__|__mro__|__subclasses__")
EOF
  done
#   cat <<EOF
#     - action: reject
#       expression: |
#         (.REQUEST.BODY
#         | has("hostnames"))
# EOF
}

##########################
# MAIN LOOP: TEST RUNS WITH DIFFERENT PARAMETERS
##########################
result_trace_file="${BASE_RESULT_DIR}/requests_trace.txt"
for test in ${TESTS[@]}; do
    for namespace in ${NAMESPACES[@]}; do
        max_num_rules=0
        if [[ $test == "enforce" ]]; then
            kubectl patch configmap config-deployment \
                -n knative-serving \
                --type merge \
                -p '{"data": {"queue-sidecar-image": "'$ENFORCER_QUEUE'"}}'
            max_num_rules=$MAX_RULES
        else
            kubectl patch configmap config-deployment \
                -n knative-serving \
                --type merge \
                -p '{"data": {"queue-sidecar-image": "'$BASELINE_QUEUE'"}}'
        fi

        for num_rules in $(seq 0 $RULES_JUMP_SIZE $max_num_rules); do
            ##########################
            # 1. REBOOT CLUSTER MACHINES AND WAIT FOR CLUSTER TO BE READY
            ##########################
            reboot_machines
            wait_for_cluster

            if [[ $num_rules -eq 0 ]]; then
                yq -i '
                .spec.template.metadata.annotations |= {} |
                .spec.template.metadata.annotations."autoscaling.knative.dev/min-scale" = "1" |
                .spec.template.metadata.annotations."autoscaling.knative.dev/max-scale" = "1" |
                .spec.template.metadata.annotations."qpoption.knative.dev/firewall-activate" = "enable" |
                del(.spec.template.metadata.annotations."qpoption.knative.dev/firewall-config-rules")
                ' sample-app/kubernetes.yaml
            else
                # $(( num_rules - 1 ))
                RULES="$(generate_rules)" \
                yq -i '
                .spec.template.metadata.annotations |= {} |
                .spec.template.metadata.annotations."autoscaling.knative.dev/min-scale" = "1" |
                .spec.template.metadata.annotations."autoscaling.knative.dev/max-scale" = "1" |
                .spec.template.metadata.annotations."qpoption.knative.dev/firewall-activate" = "enable" |
                .spec.template.metadata.annotations."qpoption.knative.dev/firewall-config-rules" = strenv(RULES)
                ' sample-app/kubernetes.yaml
            fi

            test_timestamp=$(date +%Y%m%d%H%M%S)
            test_dir="${BASE_RESULT_DIR}/test-${test}_rules-${num_rules}_namespace-${namespace}_${test_timestamp}"
            mkdir -p "$test_dir"

            while true; do
                kubectl create namespace $namespace
                if [ "$?" -eq 0 ]; then
                    break
                else
                    echo "Namespace not created with success; trying again..."
                    sleep 60
                fi
            done

            cd $namespace/
            kubectl apply -f kubernetes.yaml
            cd ..

            echo "Waiting for application to be ready (only one pod starting with 'result')..."
            needed_pods=1
            while true; do
                # Count running pods
                number_running=$(kubectl get pods -n "$namespace" | grep -c 'Running')
                # Count terminating pods
                number_terminating=$(kubectl get pods -n "$namespace" | grep -c 'Terminating')

                if [[ $number_running -eq $needed_pods && $number_terminating -eq 0 ]]; then
                    echo "Application is ready: $number_running running pods, no terminating pods."
                    break
                else
                    echo "Waiting: $number_running running pods, $number_terminating terminating pods (need $needed_pods running). Retrying in 5 seconds..."
                    sleep 5
                fi
            done

            sleep 60

            echo "Starting tests"
            for (( i=1; i<=NUMBER_TESTS; i++ )); do
                data='{"hostnames": ["test"]}'
                curl http://$ENTRY_POINT.$namespace.$EXTERNAL_IP.sslip.io --data "$data" -H 'Content-Type: application/json' -v

                sleep "$WAIT_PERIOD"
            done

            sleep 60

            echo "Saving logs from pods"
            pods_to_log=$(kubectl get pods -n "$namespace" --no-headers -o custom-columns=NAME:.metadata.name || true)
            for pod in $pods_to_log; do
                pod_log_file_queue="${test_dir}/pod_${pod}_queue_proxy_logs.txt"
                echo "Saving logs for pod $pod and container queue-proxy to $pod_log_file_queue"
                kubectl logs "$pod" -c queue-proxy -n "$namespace" > "$pod_log_file_queue"

                pod_log_file_user="${test_dir}/pod_${pod}_user_container_logs.txt"
                echo "Saving logs for pod $pod and container user-container to $pod_log_file_user"
                kubectl logs "$pod" -c user-container -n "$namespace" > "$pod_log_file_user"

                # Check if the pod has a previous instance and save its logs
                # echo "Saving logs for previous instance of pod $pod" >> "$pod_log_file"
                # kubectl logs "$pod" -c user-container -n "$namespace" --previous >> "$pod_log_file"
            done

            git add .
            git commit -s -m "new latency results for $test"
            git push

            # REMOVE EVERYTHING BEFORE NEXT ITERATION
            cd $namespace/
            kubectl delete -f kubernetes.yaml
            if [[ $namespace == "long-sequence" || $namespace == "long-parallel" ]]; then
                kubectl delete -f functions.yaml
            fi
            cd ..
            kubectl delete namespace $namespace
        done
    done
done

echo "All tests completed."
