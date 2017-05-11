CLI
===

To start the maestro API or Worker you should use one of the commands available in the CLI.

## API

```
Usage:
  maestro start [flags]

Flags:
  -b, --bind string         bind address (default "0.0.0.0")
      --context string      kubeconfig context
      --incluster           incluster mode (for running on kubernetes)
      --kubeconfig string   path to the kubeconfig file (not needed if using --incluster) (default "<homedir>/.kube/config")
  -p, --port int            bind port (default 8080)

Global Flags:
  -c, --config string   config file (default is ./config/local.yaml) (default "./config/local.yaml")
  -j, --json            json output mode
  -v, --verbose int     Verbosity level => v0: Error, v1=Warning, v2=Info, v3=Debug
```

## Worker

```
Usage:
  maestro worker [flags]

Flags:
      --context string      kubeconfig context
      --incluster           incluster mode (for running on kubernetes)
      --kubeconfig string   path to the kubeconfig file (not needed if using --incluster) (default "<homedir>/.kube/config")

Global Flags:
  -c, --config string   config file (default is ./config/local.yaml) (default "./config/local.yaml")
  -j, --json            json output mode
  -v, --verbose int     Verbosity level => v0: Error, v1=Warning, v2=Info, v3=Debug
```
