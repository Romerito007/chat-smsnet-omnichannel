# S3 bucket CORS for browser-direct uploads

Attachments use **presigned S3 URLs**: the browser uploads bytes directly to the
bucket (`PUT`) and downloads them (`GET`) — the API never proxies the bytes. A
`PUT` with a `Content-Type` header is a *non-simple* cross-origin request, so the
browser first sends a preflight `OPTIONS` to S3. If the bucket has **no CORS
policy**, S3 answers the preflight without an `Access-Control-Allow-Origin`
header and the browser blocks the upload:

```
Access to fetch at 'https://<bucket>.s3.amazonaws.com/...' from origin
'http://localhost:3000' has been blocked by CORS policy: Response to preflight
request doesn't pass access control check: No 'Access-Control-Allow-Origin'
header is present on the requested resource.
```

This is a **bucket/infra** condition, not a frontend bug — the upload-url
contract, the presigned `PUT`, and the replayed `Content-Type` header are all
correct.

## Option A — automatic (default)

On boot the API applies this CORS policy to the bucket itself
(`S3AttachmentStorage.EnsureCORS` → `PutBucketCors`), using
`S3_CORS_ALLOWED_ORIGINS` (default: `HTTP_ALLOWED_ORIGINS`).

- Toggle: `S3_ENSURE_CORS=true` (default) / `false`.
- Origins: `S3_CORS_ALLOWED_ORIGINS=http://localhost:3000,https://app.example.com`
- Requires the IAM principal to have **`s3:PutBucketCORS`** on the bucket.
- Best-effort: if the permission is missing it only logs a warning
  (`could not apply S3 bucket CORS`) and startup continues — apply Option B then.

## Option B — manual (one-off, via AWS CLI)

Edit `deploy/s3-cors.json` (replace the production/preview origins) and run:

```bash
aws s3api put-bucket-cors \
  --bucket aw-wasend-arquivos \
  --cors-configuration file://deploy/s3-cors.json
```

Verify:

```bash
aws s3api get-bucket-cors --bucket aw-wasend-arquivos
```

## Option C — avoid S3 CORS entirely (same-origin uploads)

Set `STORAGE_PROVIDER=local`. The local backend issues a **same-origin** signed
`PUT /v1/attachments/blobs/{token}` URL through the API, so the browser never
talks to S3 and no bucket CORS is needed. (S3 remains the recommended production
backend; this is the fallback when bucket CORS can't be configured.)
