# icinga_compat_log
golang icinga compat log to nats streaming POC

1. icinga2 servers with compatlog enabled

2. producer read logfile and send event to nats-streaming-server

3. consumer reads from nats-streaming-server and send event to elasticsearch
