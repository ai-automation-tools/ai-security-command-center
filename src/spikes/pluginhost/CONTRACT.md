# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [plugin.proto](#plugin-proto)
    - [DescribeRequest](#ascc-plugin-v1alpha1-DescribeRequest)
    - [DescribeResponse](#ascc-plugin-v1alpha1-DescribeResponse)
    - [HealthRequest](#ascc-plugin-v1alpha1-HealthRequest)
    - [HealthResponse](#ascc-plugin-v1alpha1-HealthResponse)
    - [Nhi](#ascc-plugin-v1alpha1-Nhi)
    - [PlatformCallRequest](#ascc-plugin-v1alpha1-PlatformCallRequest)
    - [PlatformCallResponse](#ascc-plugin-v1alpha1-PlatformCallResponse)
    - [ReportFindingRequest](#ascc-plugin-v1alpha1-ReportFindingRequest)
    - [ReportFindingResponse](#ascc-plugin-v1alpha1-ReportFindingResponse)
    - [SyncRequest](#ascc-plugin-v1alpha1-SyncRequest)
    - [SyncResponse](#ascc-plugin-v1alpha1-SyncResponse)
    - [UpsertNhiRequest](#ascc-plugin-v1alpha1-UpsertNhiRequest)
    - [UpsertNhiResponse](#ascc-plugin-v1alpha1-UpsertNhiResponse)
  
    - [Connector](#ascc-plugin-v1alpha1-Connector)
    - [Host](#ascc-plugin-v1alpha1-Host)
    - [Plugin](#ascc-plugin-v1alpha1-Plugin)
  
- [Scalar Value Types](#scalar-value-types)



<a name="plugin-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## plugin.proto
The ASCC plugin contract, v1alpha1 (FR-PLG-01, ADR-15). A spike: nothing here is stable yet.

Two directions. The host calls the plugin (Plugin, Connector). The plugin calls the host (Host),
presenting the capability token the host gave it at start. No call returns the credential the host
uses, and no field lets a plugin say who it is: the host takes that from the token.

Field tags are never reused. A removed field is reserved.


<a name="ascc-plugin-v1alpha1-DescribeRequest"></a>

### DescribeRequest







<a name="ascc-plugin-v1alpha1-DescribeResponse"></a>

### DescribeResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| plugin_id | [string](#string) |  | The plugin&#39;s id, as in its manifest. |
| version | [string](#string) |  | The plugin&#39;s own version. |
| type | [string](#string) |  | The plugin type. Only &#34;connector&#34; exists in v1alpha1. |
| contract_version | [string](#string) |  | The contract version the plugin was built against, for example &#34;v1alpha1&#34;. |






<a name="ascc-plugin-v1alpha1-HealthRequest"></a>

### HealthRequest







<a name="ascc-plugin-v1alpha1-HealthResponse"></a>

### HealthResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ok | [bool](#bool) |  | True when the plugin can do its work. |
| detail | [string](#string) |  | One line for a person. Never a secret. |






<a name="ascc-plugin-v1alpha1-Nhi"></a>

### Nhi
Nhi is a non-human identity as a connector sees it. There is no field for a secret value.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| external_id | [string](#string) |  | The platform&#39;s own id for the identity. |
| kind | [string](#string) |  | service_account, bot, ci_job, oauth_app or mcp_server. |
| display_name | [string](#string) |  |  |
| owner | [string](#string) |  | Empty when the platform names no owner. |






<a name="ascc-plugin-v1alpha1-PlatformCallRequest"></a>

### PlatformCallRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| credential_kind | [string](#string) |  | The kind of credential the host should use, for example &#34;github_app&#34;. |
| method | [string](#string) |  | Only GET in v1alpha1: connectors are read-only (ADR-16). |
| url | [string](#string) |  |  |






<a name="ascc-plugin-v1alpha1-PlatformCallResponse"></a>

### PlatformCallResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| status | [int32](#int32) |  |  |
| body | [bytes](#bytes) |  |  |






<a name="ascc-plugin-v1alpha1-ReportFindingRequest"></a>

### ReportFindingRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| rule_id | [string](#string) |  | The rule that fired, for example &#34;nhi-no-owner&#34;. |
| subject | [string](#string) |  | The external_id of the identity it is about. |
| message | [string](#string) |  | One line for a person. Never a secret. |






<a name="ascc-plugin-v1alpha1-ReportFindingResponse"></a>

### ReportFindingResponse







<a name="ascc-plugin-v1alpha1-SyncRequest"></a>

### SyncRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| cursor | [string](#string) |  | Where the last sync stopped. Empty on the first one. |






<a name="ascc-plugin-v1alpha1-SyncResponse"></a>

### SyncResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| cursor | [string](#string) |  | Where this sync stopped, to pass to the next one. |
| upserted | [int32](#int32) |  | How many identities the host accepted. |
| refused | [int32](#int32) |  | How many calls to the host were refused. |






<a name="ascc-plugin-v1alpha1-UpsertNhiRequest"></a>

### UpsertNhiRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| nhi | [Nhi](#ascc-plugin-v1alpha1-Nhi) |  |  |






<a name="ascc-plugin-v1alpha1-UpsertNhiResponse"></a>

### UpsertNhiResponse






 

 

 


<a name="ascc-plugin-v1alpha1-Connector"></a>

### Connector
Connector is served by a plugin of type &#34;connector&#34;: it reads the non-human identities of one platform.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| Sync | [SyncRequest](#ascc-plugin-v1alpha1-SyncRequest) | [SyncResponse](#ascc-plugin-v1alpha1-SyncResponse) | Sync reads the platform from the cursor on. The plugin reports what it finds by calling Host. |


<a name="ascc-plugin-v1alpha1-Host"></a>

### Host
Host is served by the plugin host, one endpoint per plugin. Every call needs the capability token
in the &#34;authorization&#34; metadata as &#34;Bearer &lt;token&gt;&#34;, and is refused unless the token grants it.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| UpsertNhi | [UpsertNhiRequest](#ascc-plugin-v1alpha1-UpsertNhiRequest) | [UpsertNhiResponse](#ascc-plugin-v1alpha1-UpsertNhiResponse) | UpsertNhi records a non-human identity in the plugin&#39;s own namespace. Needs scope &#34;nhi:write@self&#34;. |
| ReportFinding | [ReportFindingRequest](#ascc-plugin-v1alpha1-ReportFindingRequest) | [ReportFindingResponse](#ascc-plugin-v1alpha1-ReportFindingResponse) | ReportFinding records a finding in the plugin&#39;s own namespace. Needs scope &#34;findings:write@self&#34;. |
| PlatformCall | [PlatformCallRequest](#ascc-plugin-v1alpha1-PlatformCallRequest) | [PlatformCallResponse](#ascc-plugin-v1alpha1-PlatformCallResponse) | PlatformCall asks the host to call the platform with a credential the plugin never sees. The URL must be https on one of the plugin&#39;s approved hosts and the credential kind one it was approved for. |


<a name="ascc-plugin-v1alpha1-Plugin"></a>

### Plugin
Plugin is served by every plugin, whatever its type.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| Describe | [DescribeRequest](#ascc-plugin-v1alpha1-DescribeRequest) | [DescribeResponse](#ascc-plugin-v1alpha1-DescribeResponse) | Describe is the first call the host makes. Until the answer matches what was approved, the plugin&#39;s token is refused; a plugin whose answer doesn&#39;t match is stopped. |
| Health | [HealthRequest](#ascc-plugin-v1alpha1-HealthRequest) | [HealthResponse](#ascc-plugin-v1alpha1-HealthResponse) | Health says whether the plugin can do its work right now. |

 



## Scalar Value Types

| .proto Type | Notes | C++ | Java | Python | Go | C# | PHP | Ruby |
| ----------- | ----- | --- | ---- | ------ | -- | -- | --- | ---- |
| <a name="double" /> double |  | double | double | float | float64 | double | float | Float |
| <a name="float" /> float |  | float | float | float | float32 | float | float | Float |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="uint32" /> uint32 | Uses variable-length encoding. | uint32 | int | int/long | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="uint64" /> uint64 | Uses variable-length encoding. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum or Fixnum (as required) |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | uint32 | int | int | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum |
| <a name="sfixed32" /> sfixed32 | Always four bytes. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="bool" /> bool |  | bool | boolean | boolean | bool | bool | boolean | TrueClass/FalseClass |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. | string | String | str/unicode | string | string | string | String (UTF-8) |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. | string | ByteString | str | []byte | ByteString | string | String (ASCII-8BIT) |

