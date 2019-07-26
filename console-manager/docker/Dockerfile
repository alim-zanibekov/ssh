FROM ubuntu:16.04

ENV DEBUG=1
ENV SSH_PORT=8888
ENV FRONTEND_PORT=4000
ENV AGENT_PORT=9999
ENV WRITE_BUFFER_SIZE=1024
ENV READ_BUFFER_SIZE=1024
ENV AGENT_TERMINAL_WAIT_TIMEOUT=5

RUN mkdir -p /masteros/bin && mkdir -p /masteros/bin/static && apt-get update && apt-get -y install mc
COPY console-manager-1.0.0 /masteros/bin
COPY ./static/* /masteros/bin/static/
RUN chmod +x /masteros/bin/console-manager-1.0.0 \
    && ln -s /masteros/bin/console-manager-1.0.0 /masteros/bin/console-manager

EXPOSE 8888 9999 4000
WORKDIR /masteros/bin
CMD /masteros/bin/console-manager
