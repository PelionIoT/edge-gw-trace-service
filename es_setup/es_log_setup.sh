############ Step 1: Create the Index Template ############

# ------------ Kibana Console ------------
# PUT _template/device-trace-active-logs
# {
#   "index_patterns": "device-trace-active-logs-*",
#   "settings": {
#     "number_of_shards": 1,
#   },
#   "aliases": {
#     "device-trace-search-logs": {}
# },
#   "mappings": {
#     "properties": {
#         "id": {"type": "keyword"},
#         "device_id": {"type": "keyword"},
#         "account_id": {"type": "keyword"},
#         "timestamp": {
#                   "type": "date",
#                   "format": "strict_date_optional_time||epoch_millis"
#                   },
#         "@timestamp": {
#                   "type": "date",
#                   "format": "strict_date_optional_time||epoch_millis"
#                   },
#         "app_name": {"type": "text"},
#         "level": {"type": "text"},
#         "message": {"type": "text"},
#         "type": {"type": "text"},
#         "timestring": {
#                   "type": "date",
#                   "format": "strict_date_optional_time_nanos"
#                   },
#         "created_at": {
#                   "type": "date",
#                   "format": "strict_date_optional_time_nanos"
#                   }
#     }
#   }
# }

# ------------ Terminal ------------
curl -H "Content-Type: application/json" -XPUT "http://localhost:9200/_template/device-trace-active-logs" -d '
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
        "app_name": {"type": "text"},
        "level": {"type": "text"},
        "message": {"type": "text"},
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
'

############ Step 2: Create device-trace-active-logs-1 ############
# ------------ Kibana Console ------------
# PUT device-trace-active-logs-000001

# ------------ Terminal ------------
curl -H "Content-Type: application/json" -XPUT "http://localhost:9200/device-trace-active-logs-000001"


############ Step 3: Assign device-trace-active-logs alias to device-trace-active-logs-1 ############
# ------------ Kibana Console ------------
# PUT device-trace-active-logs-000001/_alias/device-trace-active-logs

# ------------ Terminal ------------
curl -H "Content-Type: application/json" -XPUT "http://localhost:9200/device-trace-active-logs-000001/_alias/device-trace-active-logs"