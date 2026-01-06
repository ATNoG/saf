from typing import Any
import yaml
from jsonschema import validate as json_validate, ValidationError
import importlib.resources as resources


with open("firewall-schema.yaml") as f:
    SCHEMA = yaml.safe_load(f)


def validate(workflow: dict[Any, Any]) -> tuple[bool, str]:
    # First, standard structural validation
    try:
        json_validate(instance=workflow, schema=SCHEMA)
    except ValidationError as e:
        return False, f"Schema validation error:\n {e}"
    return True, "Workflow passed all semantic checks"
