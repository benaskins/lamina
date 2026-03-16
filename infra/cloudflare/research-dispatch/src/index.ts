interface Env {
  WIRE_TOKEN: string;
  AI: Ai;
}

interface Task {
  url: string;
  method?: string;
  headers?: Record<string, string>;
  body?: string;
  summarize?: boolean;
}

interface TaskResult {
  url: string;
  status: number;
  body?: string;
  summary?: string;
  error?: string;
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

    if (pathname === "/dispatch") {
      return handleDispatch(request, env);
    }

    return Response.json({ error: "not found" }, { status: 404 });
  },
};

async function handleDispatch(request: Request, env: Env): Promise<Response> {
  let tasks: Task[];
  try {
    const body = await request.json();
    tasks = (body as { tasks: Task[] }).tasks;
  } catch {
    return Response.json({ error: "invalid JSON body" }, { status: 400 });
  }

  if (!Array.isArray(tasks) || tasks.length === 0) {
    return Response.json({ error: "tasks array is required" }, { status: 400 });
  }

  if (tasks.length > 20) {
    return Response.json({ error: "max 20 tasks per dispatch" }, { status: 400 });
  }

  const results = await Promise.all(tasks.map((task) => executeTask(task, env)));
  return Response.json({ results });
}

async function executeTask(task: Task, env: Env): Promise<TaskResult> {
  try {
    const resp = await fetch(task.url, {
      method: task.method || "GET",
      headers: task.headers || {},
      body: task.method && task.method !== "GET" && task.method !== "HEAD" ? task.body : undefined,
    });

    const body = await resp.text();
    const truncated = body.length > 50000 ? body.slice(0, 50000) : body;

    let summary: string | undefined;
    if (task.summarize && truncated.length > 0) {
      try {
        const ai = await env.AI.run("@cf/qwen/qwen3-30b-a3b-fp8", {
          messages: [
            { role: "user", content: `Summarize the following content concisely:\n\n${truncated}` },
          ],
          max_tokens: 500,
        });
        summary = (ai as { response: string }).response;
      } catch {
        summary = undefined;
      }
    }

    return { url: task.url, status: resp.status, body: truncated, summary };
  } catch (err) {
    return { url: task.url, status: 0, error: (err as Error).message };
  }
}
