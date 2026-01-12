#!/usr/bin/env bash

num_rules=10

generate_rules() {
  cat <<EOF
request:
  default-action: accept
  rules:
EOF

  for i in $(seq 1 "$1"); do
    cat <<EOF
    - action: expression
      expression: |
        ([.REQUEST.BODY.username?] | map(tostring) | join(" ")) | test("{{|}}|{%-|-%}|__class__|__mro__|__subclasses__")
EOF
  done
  cat <<EOF
    - action: reject
      expression: |
        (.REQUEST.BODY
        | has("hostnames"))
EOF
}

RULES="$(generate_rules $(( num_rules - 9 )))" \
yq -i '
  .spec.template.metadata.annotations |= {} |
  .spec.template.metadata.annotations."qpoption.knative.dev/firewall-config-rules" = strenv(RULES)
' sample-app/kubernetes.yaml
