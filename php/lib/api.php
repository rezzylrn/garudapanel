<?php
// GarudaPanel PHP frontend — Go API client
// Simple cURL wrapper with session token handling.

function gp_api(string $method, string $path, $body = null, ?string $token = null): array {
    $ch = curl_init(GP_API_URL . $path);
    $headers = ['Content-Type: application/json'];
    if ($token) {
        $headers[] = 'Authorization: Bearer ' . $token;
    }
    curl_setopt_array($ch, [
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_CUSTOMREQUEST => $method,
        CURLOPT_HTTPHEADER => $headers,
        CURLOPT_TIMEOUT => 30,
    ]);
    if ($body !== null) {
        curl_setopt($ch, CURLOPT_POSTFIELDS, is_string($body) ? $body : json_encode($body));
    }
    $resp = curl_exec($ch);
    $code = curl_getinfo($ch, CURLINFO_HTTP_CODE);
    $err = curl_error($ch);
    // NOTE: curl_close() intentionally omitted — deprecated in PHP 8.5,
    // handle is freed automatically at end of scope.
    if ($resp === false) {
        return ['ok' => false, 'code' => 0, 'error' => 'Connection failed: ' . $err, 'data' => null];
    }
    $data = json_decode($resp, true);
    return [
        'ok' => $code >= 200 && $code < 300,
        'code' => $code,
        'error' => (!$data || $code >= 400) ? ($data['error'] ?? "HTTP $code") : null,
        'data' => $data,
    ];
}

// gp_raw: request/response mentah (bukan JSON) — untuk baca/tulis isi file.
// Mengembalikan ['ok'=>bool, 'code'=>int, 'body'=>string, 'error'=>?string].
function gp_raw(string $method, string $path, ?string $token = null, ?string $body = null, ?string $contentType = null): array {
    $ch = curl_init(GP_API_URL . $path);
    $headers = [];
    if ($contentType) $headers[] = 'Content-Type: ' . $contentType;
    if ($token) $headers[] = 'Authorization: Bearer ' . $token;
    curl_setopt_array($ch, [
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_CUSTOMREQUEST => $method,
        CURLOPT_HTTPHEADER => $headers,
        CURLOPT_TIMEOUT => 60,
    ]);
    if ($body !== null) {
        curl_setopt($ch, CURLOPT_POSTFIELDS, $body);
    }
    $resp = curl_exec($ch);
    $code = curl_getinfo($ch, CURLINFO_HTTP_CODE);
    $err = curl_error($ch);
    // NOTE: curl_close() intentionally omitted — deprecated in PHP 8.5.
    if ($resp === false) {
        return ['ok' => false, 'code' => 0, 'body' => '', 'error' => 'Connection failed: ' . $err];
    }
    return ['ok' => $code >= 200 && $code < 300, 'code' => $code, 'body' => $resp, 'error' => $code >= 400 ? "HTTP $code" : null];
}

function gp_token(): ?string {
    return $_SESSION['gp_token'] ?? null;
}

function gp_user(): ?array {
    return $_SESSION['gp_user'] ?? null;
}

function gp_is_admin(): bool {
    return (bool)($_SESSION['gp_user']['is_admin'] ?? false);
}

// HTML escaping helper
function e(?string $s): string {
    return htmlspecialchars($s ?? '', ENT_QUOTES, 'UTF-8');
}
