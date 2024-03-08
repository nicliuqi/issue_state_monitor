FROM openeuler/openeuler:23.03 as BUILDER
RUN dnf update -y && \
    dnf install -y golang && \
    go env -w GOPROXY=https://goproxy.cn,direct

MAINTAINER liuqi<469227928@qq.com>

# build binary
WORKDIR /go/src/github.com/nicliuqi/issue_state_monitor
COPY . .
RUN GO111MODULE=on CGO_ENABLED=0 go build -a -o issue_state_monitor -buildmode=pie --ldflags "-s -linkmode 'external' -extldflags '-Wl,-z,now'" .

# copy binary config and utils
FROM openeuler/openeuler:22.03
RUN dnf -y update && \
    dnf in -y shadow && \
    dnf remove -y gdb-gdbserver && \
    groupadd -g 1000 monitor && \
    useradd -u 1000 -g monitor -s /sbin/nologin -m monitor && \
    echo > /etc/issue && echo > /etc/issue.net && echo > /etc/motd && \
    mkdir /home/monitor -p && \
    chmod 700 /home/monitor && \
    chown monitor:monitor /home/monitor && \
    echo 'set +o history' >> /root/.bashrc && \
    sed -i 's/^PASS_MAX_DAYS.*/PASS_MAX_DAYS   90/' /etc/login.defs && \
    rm -rf /tmp/*

USER monitor

WORKDIR /opt/app

COPY  --chown=monitor --from=BUILDER /go/src/github.com/nicliuqi/issue_state_monitor/issue_state_monitor /opt/app/issue_state_monitor

RUN chmod 550 /opt/app/issue_state_monitor && \
    echo "umask 027" >> /home/monitor/.bashrc && \
    echo 'set +o history' >> /home/monitor/.bashrc

ENTRYPOINT ["/opt/app/issue_state_monitor"]
