## `syslog` sink

The `syslog` sink provides an easy way to submit metrics and events to the Syslog logging system.

### Configuration structure

```json
{
  "<name>": {
    "type": "syslog",
    "meta_as_tags" : [],
    "syslog_tag" : "mytag",
    "syslog_priorities" : [
        "LOG_INFO",
        "LOG_LOCAL7",
    ],
    "syslog_writer" : "info"
  }
}
```

- `type`: makes the sink an `syslog` sink
- `meta_as_tags`: print meta information as tags in the output (optional)
- `syslog_tag`: The tag submitted with each message
- `syslog_priorities`: List of syslog priorities. Will be OR'd together
- `syslog_writer`: Submit metrics with this write level


#### Possible priorities for `syslog_priorities`
- `LOG_EMERG`
- `LOG_ALERT`
- `LOG_CRIT`
- `LOG_ERR`
- `LOG_NOTICE`
- `LOG_INFO`
- `LOG_DEBUG`
- `LOG_USER`
- `LOG_MAIL`
- `LOG_DAEMON`
- `LOG_AUTH`
- `LOG_SYSLOG`
- `LOG_LPR`
- `LOG_NEWS`
- `LOG_UUCP`
- `LOG_CRON`
- `LOG_AUTHPRIV`
- `LOG_FTP`
- `LOG_LOCAL0`
- `LOG_LOCAL1`
- `LOG_LOCAL2`
- `LOG_LOCAL3`
- `LOG_LOCAL4`
- `LOG_LOCAL5`
- `LOG_LOCAL6`
- `LOG_LOCAL7`

#### Possible writers for `syslog_writer`
- `info`
- `debug`
- `alert`
- `emerg`
- `err`
- `notice`
- `warning`