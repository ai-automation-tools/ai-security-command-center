# Development only. Plain HTTP and a one-node store: the Compose file publishes the port on loopback alone.
ui           = true
api_addr     = "http://127.0.0.1:8200"
cluster_addr = "http://127.0.0.1:8201"

storage "raft" {
  path    = "/openbao/file/data"
  node_id = "dev"
}

listener "tcp" {
  address     = "0.0.0.0:8200"
  tls_disable = true
}

# Unseals on start with the key from .env: 32 random bytes, written as 64 hex digits.
seal "static" {
  current_key_id = "dev-1"
  current_key    = "env://ASCC_OPENBAO_SEAL_KEY"
}
