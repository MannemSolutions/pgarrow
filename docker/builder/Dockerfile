FROM rockylinux:8
ENV GOVER=1.19.3

COPY scripts/* /scripts/
RUN dnf update -y && \
    dnf groupinstall -y "Development Tools" && \
   dnf install -y bind-utils make git iproute

RUN /scripts/install_go.sh
CMD /scripts/start.sh
