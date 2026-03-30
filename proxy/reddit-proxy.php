<?php
// Simple Reddit JSON proxy for bypassing datacenter IP blocks.
// Deploy to a server with a non-blocked IP.
//
// Usage: Send GET request with headers X-Feed-ID and X-Feed-User
//
// Configure feed-forge to use this proxy URL instead of reddit.com directly.

// Shared secret - change this and set the same value in feed-forge config
$secret = getenv('REDDIT_PROXY_SECRET') ?: 'CHANGE_ME';

// Verify the shared secret
$provided = $_SERVER['HTTP_X_PROXY_SECRET'] ?? '';
if (!hash_equals($secret, $provided)) {
    http_response_code(403);
    echo json_encode(['error' => 'forbidden']);
    exit;
}

$feed = $_SERVER['HTTP_X_FEED_ID'] ?? '';
$user = $_SERVER['HTTP_X_FEED_USER'] ?? '';

if ($feed === '' || $user === '') {
    http_response_code(400);
    echo json_encode(['error' => 'missing X-Feed-ID or X-Feed-User header']);
    exit;
}

$url = 'https://www.reddit.com/.json?' . http_build_query([
    'feed' => $feed,
    'user' => $user,
]);

$ch = curl_init($url);
curl_setopt_array($ch, [
    CURLOPT_RETURNTRANSFER => true,
    CURLOPT_FOLLOWLOCATION => true,
    CURLOPT_TIMEOUT        => 30,
    CURLOPT_USERAGENT      => 'FeedForge/1.0 (by /u/feedforge)',
    CURLOPT_HTTPHEADER     => ['Accept: application/json'],
]);

$body = curl_exec($ch);
$status = curl_getinfo($ch, CURLINFO_HTTP_CODE);
$err = curl_error($ch);
curl_close($ch);

if ($err !== '') {
    http_response_code(502);
    echo json_encode(['error' => 'upstream request failed', 'detail' => $err]);
    exit;
}

http_response_code($status);
header('Content-Type: application/json');
echo $body;
