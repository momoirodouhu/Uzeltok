FROM gcr.io/distroless/static-debian13:nonroot
ARG TARGETPLATFORM

WORKDIR /app

COPY $TARGETPLATFORM/Uzeltok-server ./Uzeltok-server

USER nonroot

EXPOSE 8080

ENTRYPOINT ["./Uzeltok-server"]
