import yaml
import json

f = open('capsule-template.yaml')
dataMap = yaml.load(f)
json = json.dumps(dataMap)
f.close()
print json
