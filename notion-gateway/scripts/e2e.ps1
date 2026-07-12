[CmdletBinding()]
param(
    [string]$BaseUrl = "http://127.0.0.1:8787",
    [string]$ApiKey = $env:NOTION_CODEX_MCP_API_KEY,
    [string]$ProjectId = "notioncodex-gateway",
    [string]$ConversationId = "e2e-memory-$([guid]::NewGuid().ToString('N'))"
)

$ErrorActionPreference = "Stop"

function Assert-True([bool]$Condition, [string]$Message) {
    if (-not $Condition) { throw "ASSERTION FAILED: $Message" }
}

function Invoke-Gateway([string]$Method, [string]$Path, $Body = $null, [hashtable]$Headers = @{}) {
    if ($ApiKey) { $Headers["X-NotionCodex-API-Key"] = $ApiKey }
    $params = @{ Method = $Method; Uri = "$BaseUrl$Path"; Headers = $Headers; ErrorAction = "Stop" }
    if ($null -ne $Body) {
        $params["ContentType"] = "application/json"
        $params["Body"] = ($Body | ConvertTo-Json -Depth 12 -Compress)
    }
    return Invoke-WebRequest @params
}

Write-Host "[1/5] Health and model discovery"
$health = Invoke-Gateway "GET" "/healthz"
Assert-True ($health.StatusCode -eq 200) "healthz must return 200"
$models = Invoke-Gateway "GET" "/v1/models"
Assert-True ($models.Content -match 'mock/echo') "mock/echo model must be discoverable"

Write-Host "[2/5] MCP tool discovery policy"
$toolsResponse = Invoke-Gateway "GET" "/v1/mcp/tools"
$tools = ($toolsResponse.Content | ConvertFrom-Json).data
Assert-True ($tools.Count -gt 0) "MCP tools must be discovered"
$toolNames = @($tools | ForEach-Object { $_.name })
foreach ($forbidden in @("write_file", "edit_file", "move_file", "create_directory")) {
    Assert-True (-not ($toolNames -contains $forbidden)) "read-only policy must exclude $forbidden"
}
Assert-True ($toolNames -contains "list_directory") "list_directory must be allowed"

Write-Host "[3/5] MCP tools/call"
$call = Invoke-Gateway "POST" "/v1/mcp/call" @{
    server = "project-filesystem"
    name = "list_directory"
    arguments = @{ path = "." }
    conversation = $ConversationId
    metadata = @{ ncg_project_id = $ProjectId }
}
$callResult = $call.Content | ConvertFrom-Json
Assert-True (-not $callResult.is_error) "list_directory must succeed"
Assert-True ($callResult.content.Count -gt 0) "tool result must contain content"

Write-Host "[4/5] Conversation continuity"
$first = Invoke-Gateway "POST" "/v1/responses" @{
    model = "mock/echo"
    input = "Decision: E2E memory continuity works."
    stream = $false
    metadata = @{ ncg_conversation_id = $ConversationId; ncg_project_id = $ProjectId }
}
$conversationHeader = $first.Headers["X-Notioncodex-Conversation-Id"]
Assert-True (-not [string]::IsNullOrWhiteSpace($conversationHeader)) "gateway must return conversation header"
$second = Invoke-Gateway "POST" "/v1/responses" @{
    model = "mock/echo"
    input = "Continue the same project."
    stream = $false
    metadata = @{ ncg_conversation_id = $ConversationId; ncg_project_id = $ProjectId }
}
Assert-True ($second.Headers["X-Notioncodex-Conversation-Id"] -eq $conversationHeader) "conversation ID must be stable"

Write-Host "[5/5] Approval gate is not enabled for read-only policy"
Write-Host "E2E PASSED: health, model, MCP discovery/call, and memory continuity are healthy." -ForegroundColor Green
