import yaml
from validation import validate

def main():
    with open("example.yaml") as f:
        example = yaml.safe_load(f)
    validate(example)
    

if __name__ == "__main__":
    main()
