<?php
// Generic Reddit URL proxy for fetching OpenGraph metadata from reddit domains.
// Deploy alongside reddit-proxy.php on a server with a non-blocked IP.
//
// Usage: Send GET request with headers:
//   X-Proxy-Secret: shared secret
//   X-Target-URL: full reddit URL to fetch

// Shared secret - change this and set the same value in feed-forge config
$secret = getenv('REDDIT_PROXY_SECRET') ?: 'CHANGE_ME';

// Verify the shared secret
$provided = $_SERVER['HTTP_X_PROXY_SECRET'] ?? '';
if (!hash_equals($secret, $provided)) {
    http_response_code(403);
    echo json_encode(['error' => 'forbidden']);
    exit;
}

$targetURL = $_SERVER['HTTP_X_TARGET_URL'] ?? '';

if ($targetURL === '') {
    http_response_code(400);
    echo json_encode(['error' => 'missing X-Target-URL header']);
    exit;
}

// Only allow proxying reddit domains
$parsed = parse_url($targetURL);
$host = $parsed['host'] ?? '';
$allowedDomains = ['reddit.com', 'www.reddit.com', 'old.reddit.com', 'i.redd.it', 'v.redd.it', 'redd.it'];
$allowed = false;
foreach ($allowedDomains as $domain) {
    if ($host === $domain || str_ends_with($host, '.' . $domain)) {
        $allowed = true;
        break;
    }
}

if (!$allowed) {
    http_response_code(400);
    echo json_encode(['error' => 'only reddit domains are allowed']);
    exit;
}

$ch = curl_init($targetURL);
curl_setopt_array($ch, [
    CURLOPT_RETURNTRANSFER => true,
    CURLOPT_FOLLOWLOCATION => true,
    CURLOPT_TIMEOUT        => 15,
    CURLOPT_USERAGENT      => 'Mozilla/5.0 (compatible; FeedForge/1.0; OpenGraph fetcher)',
    CURLOPT_HTTPHEADER     => [
        'Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8',
        'Accept-Language: en-US,en;q=0.5',
    ],
]);

$body = curl_exec($ch);
$status = curl_getinfo($ch, CURLINFO_HTTP_CODE);
$contentType = curl_getinfo($ch, CURLINFO_CONTENT_TYPE);
$err = curl_error($ch);
curl_close($ch);

if ($err !== '') {
    http_response_code(502);
    echo json_encode(['error' => 'upstream request failed', 'detail' => $err]);
    exit;
}

http_response_code($status);
if ($contentType) {
    header('Content-Type: ' . $contentType);
}
echo $body;
