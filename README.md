# PRTools

## Todo

- [ ] framework
- [ ] gerrit module
- [ ] github module
- [ ] git module

## Summary

开启两个监听端口，处理github的webhook和gerrit的webhook。

开启gerrit的merge commit，接收到github的pull request后，创建merge commit，并生成change-id，记录在数据库中。

## Database Design

use the sqlite:

```text
+++++++++++++++++++++++++++++++++++++++++++++
+ github id | gerrit id | change-id | state +
+++++++++++++++++++++++++++++++++++++++++++++
```

```text
gerrit: ''
repo_dir: './tmp/'
database:
  - type: json
  - filename: database.json
auth:
  github:
    - token: ''
  gerrit:
    - token: ''
```

## Test

```shell
curl -X POST --data "@./pr.json" http://127.0.0.1:3002/ -H "X-GitHub-Event: pull_request" -H "content-type: application/json"
```
