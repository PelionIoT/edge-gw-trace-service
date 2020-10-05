# edge-gw-trace-service

### Introduction

This is a cloud service for collecting and querying device trace logs from edge gateways. It uses [Elasticsearch](https://www.elastic.co/downloads/elasticsearch) to store and search the trace data, [gorilla/mux](https://github.com/gorilla/mux) for routing service.

### Local Usage

- Setup the [Elasticsearch](https://www.elastic.co/downloads/elasticsearch)

- Initial the data schema

> **Note:** Go to the es_setup folder, run the script or configure it by [Kibana](https://www.elastic.co/downloads/kibana)). See the following example for template device-trace-active-logs-*

```
{
  "index_patterns": "device-trace-active-logs-*",
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0,
    "routing.allocation.total_shards_per_node": 1
  },
  "aliases": {
    "device-trace-search-logs": {}
},
  "mappings": {
    "properties": {
        "id": {"type": "keyword"},
        "device_id": {"type": "keyword"},
        "account_id": {"type": "keyword"},
        "timestamp": {
                  "type": "date",
                  "format": "strict_date_optional_time||epoch_millis"
                  },
        "@timestamp": {
                  "type": "date",
                  "format": "strict_date_optional_time||epoch_millis"
                  },
        "trace": {"type": "object"},
        "type": {"type": "text"},
        "timestring": {
                  "type": "date",
                  "format": "strict_date_optional_time_nanos"
                  },
        "created_at": {
                  "type": "date",
                  "format": "strict_date_optional_time_nanos"
                  }
    }
  }
}
```

- Setup dependencies

### Configuration
| Environment Variable | Type   | Description           | Example |
| -------------------- | ------ | --------------------- | ------- | 
| esURL | string | The host address for elastic search service | http://es.minikube:32755 |
| esSearchAlias | string | The search alias name for the elastic search service | device-trace-search-logs | 
| esActiveAlias | string | The active alias name for the elastic search service | device-trace-active-logs |
| loggingLevel | string | The lowest logging level that want to print out | debug |
| uuidNetworkInterface | string | The network interface to be used for uuid generation | eth0 |
| jwtKey | string | The filepath to public key for access token validation | /path/to/jwtKey |
| deviceDirectoryURL | string | The URL of the device directory service | - |
| jwtIssuer | string | The issuer field for JWT tokens | gateway-trace |
| jwtExpiration | integer | The JWT expiration time in seconds | 60 |
| jwtSigningKey | string | The filepath to private key used for JWT signing | /path/to/key |
