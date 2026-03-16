interface Env {
  WIRE_TOKEN: string;
}

interface ProxyRequest {
  url: string;
  method?: string;
  headers?: Record<string, string>;
  body?: string;
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    if (request.method === "GET" && new URL(request.url).pathname === "/health") {
      return Response.json({ status: "healthy" });
    }

    if (request.method !== "POST") {
      return Response.json({ error: "method not allowed" }, { status: 405 });
    }

    const token = request.headers.get("X-Wire-Token");
    if (!token || token !== env.WIRE_TOKEN) {
      return Response.json({ error: "unauthorized" }, { status: 401 });
    }

    const pathname = new URL(request.url).pathname;

    if (pathname === "/proxy") {
      return handleProxy(request);
    }

    return Response.json({ error: "not found" }, { status: 404 });
  },
};

async function handleProxy(request: Request): Promise<Response> {
  let req: ProxyRequest;
  try {
    req = await request.json();
  } catch {
    return Response.json({ error: "invalid JSON body" }, { status: 400 });
  }

  if (!req.url) {
    return Response.json({ error: "url is required" }, { status: 400 });
  }

  try {
    const url = new URL(req.url);
    if (url.protocol !== "https:" && url.protocol !== "http:") {
      return Response.json({ error: "only http/https URLs allowed" }, { status: 400 });
    }
  } catch {
    return Response.json({ error: "invalid url" }, { status: 400 });
  }

  try {
    const resp = await fetch(req.url, {
      method: req.method || "GET",
      headers: req.headers || {},
      body: req.method && req.method !== "GET" && req.method !== "HEAD" ? req.body : undefined,
    });

    const body = await resp.arrayBuffer();
    const headers = new Headers();
    headers.set("X-Proxy-Status", resp.status.toString());
    resp.headers.forEach((value, key) => {
      if (!["transfer-encoding", "content-encoding"].includes(key.toLowerCase())) {
        headers.set(key, value);
      }
    });

    return new Response(body, { status: resp.status, headers });
  } catch (err) {
    return Response.json(
      { error: "proxy fetch failed", detail: (err as Error).message },
      { status: 502 }
    );
  }
}
