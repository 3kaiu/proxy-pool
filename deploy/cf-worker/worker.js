/**
 * OpenCode Zen API Relay — Cloudflare Worker
 *
 * 原理：CF Workers 运行在全球 300+ 边缘节点，每次请求天然来自不同的出口 IP。
 * 遇到 FreeUsageLimitError 时自动重试，下一次请求大概率走不同边缘节点 = 不同 IP。
 * 相当于用 CF 的 IP 池做免费代理轮换，比公开代理池更稳定、更快。
 *
 * 部署：
 *   1. npx wrangler deploy deploy/cf-worker/wrangler.toml
 *   2. 或在 CF Dashboard -> Workers -> 新建 -> 粘贴此文件
 *
 * 使用：
 *   curl https://your-worker.workers.dev/v1/chat/completions \
 *     -H "Content-Type: application/json" \
 *     -d '{"model":"deepseek-v4-flash-free","messages":[{"role":"user","content":"Hi"}],"max_tokens":1200}'
 */

const ZEN_BASE = "https://opencode.ai/zen/v1";
const MAX_RETRIES = 8;
const RETRY_DELAY_MS = 200;

// 可选：设置 API_KEY 环境变量来保护你的 Worker 不被滥用
function checkAuth(request, env) {
  if (env.API_KEY && env.API_KEY.length > 0) {
    const auth = request.headers.get("Authorization");
    if (auth !== `Bearer ${env.API_KEY}`) {
      return new Response(JSON.stringify({ error: "Unauthorized" }), {
        status: 401,
        headers: { "Content-Type": "application/json" },
      });
    }
  }
  return null;
}

async function retryFetch(url, options, maxRetries) {
  let lastError = null;
  let lastBody = null;

  for (let i = 0; i < maxRetries; i++) {
    try {
      const resp = await fetch(url, options);

      // 非 JSON 响应直接返回（流式等）
      const contentType = resp.headers.get("content-type") || "";
      if (!contentType.includes("application/json")) {
        return resp;
      }

      const body = await resp.text();

      // 检查是否被限速
      if (body.includes("FreeUsageLimitError") || body.includes("rate_limit")) {
        lastBody = body;
        lastError = "rate_limited";
        // 等一小会儿再重试（不同边缘节点 = 不同 IP）
        await new Promise((r) => setTimeout(r, RETRY_DELAY_MS * (i + 1)));
        continue;
      }

      // 成功或其他错误，直接返回
      return new Response(body, {
        status: resp.status,
        headers: resp.headers,
      });
    } catch (err) {
      lastError = err.message;
      await new Promise((r) => setTimeout(r, RETRY_DELAY_MS * (i + 1)));
    }
  }

  // 所有重试都失败
  if (lastBody) {
    return new Response(lastBody, {
      status: 429,
      headers: { "Content-Type": "application/json" },
    });
  }
  return new Response(
    JSON.stringify({ error: `All ${maxRetries} retries failed: ${lastError}` }),
    { status: 502, headers: { "Content-Type": "application/json" } }
  );
}

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);

    // CORS 预检
    if (request.method === "OPTIONS") {
      return new Response(null, {
        headers: {
          "Access-Control-Allow-Origin": "*",
          "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
          "Access-Control-Allow-Headers": "Content-Type, Authorization",
        },
      });
    }

    // 健康检查
    if (url.pathname === "/" || url.pathname === "/health") {
      return new Response(JSON.stringify({ status: "ok", target: ZEN_BASE }), {
        headers: { "Content-Type": "application/json" },
      });
    }

    // 鉴权
    const authError = checkAuth(request, env);
    if (authError) return authError;

    // 构建目标 URL
    const targetUrl = ZEN_BASE + url.pathname + url.search;

    // 转发请求头，移除 host/origin 等
    const forwardHeaders = new Headers();
    const skipHeaders = new Set(["host", "origin", "referer", "cf-connecting-ip", "cf-ray", "cf-worker"]);
    for (const [key, value] of request.headers.entries()) {
      if (!skipHeaders.has(key.toLowerCase())) {
        forwardHeaders.set(key, value);
      }
    }

    const options = {
      method: request.method,
      headers: forwardHeaders,
    };

    // GET/HEAD 不带 body
    if (request.method !== "GET" && request.method !== "HEAD") {
      options.body = request.body;
    }

    // 带重试的请求
    const corsHeaders = {
      "Access-Control-Allow-Origin": "*",
    };

    const resp = await retryFetch(targetUrl, options, MAX_RETRIES);

    // 给响应加 CORS 头
    const newHeaders = new Headers(resp.headers);
    newHeaders.set("Access-Control-Allow-Origin", "*");

    return new Response(resp.body, {
      status: resp.status,
      headers: newHeaders,
    });
  },
};
