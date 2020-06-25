FROM ubuntu:16.04

ENV SERVER_AGENT=server:9999
ENV SERVER_SSH=server:8888
ENV SSH_PORT=:22
ENV DEBUG=1
ENV TOKEN=0000000002
ENV SSH_PASSWORD=pass2
ENV SSH_USER=root

ENV NOTVISIBLE="in users profile"

RUN apt-get update \
    && apt-get install -y openssh-server \
    && mkdir /var/run/sshd \
    && echo "root:$SSH_PASSWORD" | chpasswd \
    && sed -i 's/PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config \
    && sed 's@session\s*required\s*pam_loginuid.so@session optional pam_loginuid.so@g' -i /etc/pam.d/sshd \
    && echo "export VISIBLE=now" >> /etc/profile

RUN mkdir -p /console-agent/bin

COPY console-agent-1.0.0 /console-agent/bin
COPY console-ssh-connector-1.0.0 /console-agent/bin

RUN chmod +x /console-agent/bin/console-agent-1.0.0 \
    && chmod +x /console-agent/bin/console-ssh-connector-1.0.0 \
    && ln -s /console-agent/bin/console-agent-1.0.0 /bin/console-agent \
    && ln -s /console-agent/bin/console-ssh-connector-1.0.0 /bin/console-ssh-connector

CMD nohup /usr/sbin/sshd && /bin/console-agent $TOKEN
