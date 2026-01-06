import yaml
import json
from validation import validate

WORKFLOW = "../apps/bank/workflow/src/main/resources/workflow.sw.yaml"
KUBERNETES_CONFIG = "../apps/bank/kubernetes.yaml"

def main():
    with open(WORKFLOW) as f:
        workflow = yaml.safe_load(f)

    with open(KUBERNETES_CONFIG) as f:
        kubernetes = list(yaml.safe_load_all(f))

    functions_rules = {}
    for function in workflow["functions"]:
        if (metadata := function.get("metadata")) and (firewall := metadata.get("firewall")):
            valid, message = validate({"firewall": firewall})

            if valid:
                functions_rules[function["operation"].replace("knative:services.v1.serving.knative.dev/", "").split("?")[0]] = firewall
            else:
                raise Exception(message)

    for config in kubernetes:
        if config.get("kind") == "Service":
            for function in functions_rules:
                if config["metadata"]["name"] == function:
                    if "spec" not in config:
                        config["spec"] = {}
                    if "template" not in config["spec"]:
                        config["spec"]["template"] = {}
                    if "metadata" not in config["spec"]["template"]:
                        config["spec"]["template"]["metadata"] = {}
                    if "annotations" not in config["spec"]["template"]["metadata"]:
                        config["spec"]["template"]["metadata"]["annotations"] = {}
                    config["spec"]["template"]["metadata"]["annotations"]["qpoption.knative.dev/qpoption.knative.dev/firewall-config-rules"] = json.dumps(functions_rules[function])
                    config["spec"]["template"]["metadata"]["annotations"]["qpoption.knative.dev/qpoption.knative.dev/firewall-activate"] = "enable"
    
    with open("kubernetes.yaml", "w") as f:
        yaml.safe_dump_all(kubernetes, f, sort_keys=False)



if __name__ == "__main__":
    main()
