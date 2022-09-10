# Firemesh

**Note** This project is POC and is highly WIP.

## What is Firemesh?
Firemesh is a service that allows you to integrate Triggermesh components with Firefly. Currently Firemesh only supports the integration of Triggermesh Sources, however, Targets are in the works.

## How to add Firemesh to your Firefly Development Enviorment

After starting your Firefly development environment, you can add Firemesh by adding the following to the generated docker-compose file.

```yaml
    firemesh:
        image: gcr.io/triggermesh/firemesh
        ports:
            - 8080
        environment:
            # If no topic is provided here, firemesh will use the incoming event type to dynamically set the topic name.
            TOPIC: firemesh
            FF: http://firefly_core_0:5000
```

Now you can start adding Triggermesh Sources to your Firefly environment, and point them to the Firemesh service.

For instance, to add a Webhook or Kafka Source, you can add the following to your docker-compose file.

```yaml
    webhook:
        platform: linux/amd64
        image: gcr.io/triggermesh/webhooksource-adapter
        environment:
          # If we do not specify a static topic for firemesh, the event type we specify here will be used as the Topic name in Firefly.
          WEBHOOK_EVENT_TYPE: webhook.event
          WEBHOOK_EVENT_SOURCE: webhook
          K_SINK: "http://firemesh:8080"
        ports:
          - 8000:8080

    kafka-source:
      # platform: linux/amd64
      image: gcr.io/triggermesh/kafkasource-adapter
      environment:
        # Stream Pool Name
        TOPICS:
        # <Tennancy>/<email>/<OCID>
        USERNAME:
        # Auth Token
        PASSWORD:
        # Kafka Connection Settings
        BOOTSTRAP_SERVERS:
        GROUP_ID: ocikafka-group
        SECURITY_MECHANISMS: PLAIN
        SKIP_VERIFY: "true"
        SASL_ENABLE: "true"
        TLS_ENABLE: "true"
        K_SINK: "http://firemesh:8080"
      ports:
        - 8080
```
