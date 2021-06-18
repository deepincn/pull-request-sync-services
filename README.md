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

## WorkFlow

when create a pullrequest from github, github will call system for webhook.

webhook create a Task to task channel queue, task run some action:

- init git from gerrit and add github remote
- fetch pull/<number>/head to local branch
- merge branch and generate changeid
- push to gerrit

comment:

github上的评论会通过webhook发送到系统中，从数据库查找到pr对应的gerrit提交，

提取评论信息、文件名和行数，发送给gerrit模块，由gerrit模块将评论信息push到gerrit中。

gerrit上的评论会通过gerrit插件获取并生成webhook信息发送给系统中，提取评论信息、文件名和行数后发送给github模块，由github模块将评论信息push到github上。