#!/bin/bash

wrk -t8 -c100 -d5s -T1s --script=send_msg.lua --latency  "http://127.0.0.1:8109/msg/send_msg"
