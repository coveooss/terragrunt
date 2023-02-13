import os

from yaml import load

def main():
    script_dir = os.path.dirname(__file__)

    with open(os.path.join(script_dir, "test.yaml")) as yaml_file:
        print(load(yaml_file))

if __name__ == "__main__":
    main()
