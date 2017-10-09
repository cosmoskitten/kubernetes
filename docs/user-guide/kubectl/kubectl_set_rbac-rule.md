## kubectl set rbac-rule

Set the Rule to a role/clusterrole

### Synopsis


Add new rule existing rule of roles. 

Possible resources include (case insensitive): role, clusterrole

```
kubectl set rbac-rule (-f FILENAME | TYPE NAME) --verb=verb --resource=resource.group/subresource [--resource-name=resourcename] [--dry-run]
```

### Examples

```
  # Add the rule to role/clusterrole
  kubectl set rbac-rule role foo --resource=rs.extensions --verb=get --verb=list
  
  # Add the subresource rule to role/clusterrole
  kubectl set rbac-rule policy role foo --resource=rs.extensions/scale --verb=get,list,delete
  
  # Add the non resource rule to clusterrole
  kubectl set rbac-rule clusterrole test --non-resource-url="*" --verb=get,post,put,delete
  
  # Print the result (in yaml format) of updating role/clusterrole from a local, without hitting the server
  kubectl set rbac-rule -f path/to/file.yaml --resource=pods --verb=get --local -o yaml
```

### Options

```
      --all                            select all resources in the namespace of the specified resource types
      --allow-missing-template-keys    If true, ignore any errors in templates when a field or map key is missing in the template. Only applies to golang and jsonpath output formats. (default true)
      --dry-run                        If true, only print the object that would be sent, without sending it.
  -f, --filename stringSlice           Filename, directory, or URL to files the resource to update the rules
      --local                          If true, set resources will NOT contact api-server but run locally.
      --no-headers                     When using the default or custom-column output format, don't print headers (default print headers).
      --non-resource-url stringSlice   a set of partial urls that a user should have access to
  -o, --output string                  Output format. One of: json|yaml|wide|name|custom-columns=...|custom-columns-file=...|go-template=...|go-template-file=...|jsonpath=...|jsonpath-file=... See custom columns [http://kubernetes.io/docs/user-guide/kubectl-overview/#custom-columns], golang template [http://golang.org/pkg/text/template/#pkg-overview] and jsonpath template [http://kubernetes.io/docs/user-guide/jsonpath].
  -R, --recursive                      Process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.
      --resource stringSlice           resource that the rule applies to
      --resource-name stringSlice      resource in the white list that the rule applies to
  -l, --selector string                Selector (label query) to filter on, supports '=', '==', and '!='.
  -a, --show-all                       When printing, show all resources (default hide terminated pods.)
      --show-labels                    When printing, show all labels as the last column (default hide labels column)
      --sort-by string                 If non-empty, sort list types using this field specification.  The field specification is expressed as a JSONPath expression (e.g. '{.metadata.name}'). The field in the API resource specified by this JSONPath expression must be an integer or a string.
      --template string                Template string or path to template file to use when -o=go-template, -o=go-template-file. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview].
      --verb stringSlice               verb that applies to the resources/non-resources contained in the rule
```

### Options inherited from parent commands

```
      --alsologtostderr                  log to standard error as well as files
      --as string                        Username to impersonate for the operation
      --as-group stringArray             Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --cache-dir string                 Default HTTP cache directory (default "/home/username/.kube/http-cache")
      --certificate-authority string     Path to a cert file for the certificate authority
      --client-certificate string        Path to a client certificate file for TLS
      --client-key string                Path to a client key file for TLS
      --cluster string                   The name of the kubeconfig cluster to use
      --context string                   The name of the kubeconfig context to use
      --insecure-skip-tls-verify         If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string                Path to the kubeconfig file to use for CLI requests.
      --log-backtrace-at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log-dir string                   If non-empty, write log files in this directory
      --logtostderr                      log to standard error instead of files
      --match-server-version             Require server version to match client version
  -n, --namespace string                 If present, the namespace scope for this CLI request
      --password string                  Password for basic authentication to the API server
      --request-timeout string           The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                    The address and port of the Kubernetes API server
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
      --token string                     Bearer token for authentication to the API server
      --user string                      The name of the kubeconfig user to use
      --username string                  Username for basic authentication to the API server
  -v, --v Level                          log level for V logs
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO
* [kubectl set](kubectl_set.md)	 - Set specific features on objects

###### Auto generated by spf13/cobra on 9-Oct-2017
