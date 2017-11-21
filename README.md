# slack-relay

This is a relay server for Slack. Components:

* `slack-relay`: simply relays messages to slack via webhook integration. Clients are responsible for correctly-formed payload.
* `slack-raw`: implements a simple text-based protocol to deliver notifications to slack.

## slack-relay

Relay is intended to simplify access to slack by proxying all incoming messages to a configured webhook. Used by `slack-raw` service and `slack-send` tool.

## slack-raw

### Protocol

Simple text-based protocol, similar to SMTP and others.

**Quick start example:** 

```
echo -e "channel #monitoring\ntext Process *redis* is not running!" > /dev/tcp/slack-relay.local/8081
```

There are two types of messages `slack-raw` can send:

* simple: messages which specify `CHANNEL` and `TEXT` keywords only
* with a single attachment: specifying any keyword in addition to `CHANNEL` and `TEXT` leads to forming a message with attachment

Protocol description:

```
CHANNEL <channel-name>
LEVEL {good|bad|warning|danger|info}
TEXT
multi-line markdown message
followed by a single period
to specify end of text
.
PRETEXT single line _markdown-enabled_ text
. 
FIELD <name> <short|wide> <value>
```

**Extended example**

```bash
MSG='CHANNEL #monitoring
LEVEL danger
PRETEXT :exclamation: Backup FAILED
.
TEXT
Routine database backup failed: not enough disk space

/dev/xvda2       20G   19G    0G 100% /backups
.
FIELD Date short 2017-11-10T22:10:13Z
FIELD Host short srvdb-01
'
echo "$MGS" > /dev/tcp/$SLACK_RAW/8081
```
