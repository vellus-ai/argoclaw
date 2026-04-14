import { useState, useRef, useEffect, useCallback } from "react";
import { useTranslation } from "react-i18next";
import { Methods, PROTOCOL_VERSION } from "@/api/protocol";
import { generateId } from "@/lib/utils";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";

type PairingStatus = "idle" | "connecting" | "pending" | "approved";

interface PairingFormProps {
  onApproved: (senderID: string, userId: string) => void;
}

export function PairingForm({ onApproved }: PairingFormProps) {
  const { t } = useTranslation("login");
  const [userId, setUserId] = useState("");
  const [code, setCode] = useState<string | null>(null);
  const senderIDRef = useRef<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [status, setStatus] = useState<PairingStatus>("idle");

  const wsRef = useRef<WebSocket | null>(null);
  const pollRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (pollRef.current) clearTimeout(pollRef.current);
      if (wsRef.current) wsRef.current.close();
    };
  }, []);

  const handleApproved = useCallback(
    (sid: string) => {
      setStatus("approved");
      if (pollRef.current) clearTimeout(pollRef.current);
      if (wsRef.current) wsRef.current.close();
      onApproved(sid, userId.trim());
    },
    [userId, onApproved],
  );

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!userId.trim()) return;
    setError(null);
    setStatus("connecting");

    const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
    const wsUrl =
      import.meta.env.VITE_WS_URL || `${proto}//${window.location.host}/ws`;
    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      const id = generateId();
      ws.send(
        JSON.stringify({
          type: "req",
          id,
          method: Methods.CONNECT,
          params: {
            user_id: userId.trim(),
            protocolVersion: PROTOCOL_VERSION,
          },
        }),
      );
    };

    ws.onmessage = (event) => {
      let frame: {
        type: string;
        id?: string;
        ok?: boolean;
        payload?: Record<string, unknown>;
      };
      try {
        frame = JSON.parse(event.data as string);
      } catch {
        return;
      }

      if (frame.type !== "res" || !frame.ok || !frame.payload) return;

      const payload = frame.payload;

      if (
        payload.status === "pending_pairing" &&
        payload.pairing_code &&
        payload.sender_id
      ) {
        setCode(payload.pairing_code as string);
        const sid = payload.sender_id as string;
        senderIDRef.current = sid;
        setStatus("pending");
        startPolling(ws, sid);
        return;
      }

      if (payload.status === "approved" && senderIDRef.current) {
        handleApproved(senderIDRef.current);
      }
    };

    ws.onclose = () => {
      if (status === "pending") {
        setError(t("pairing.errorConnectionLost"));
        setStatus("idle");
        setCode(null);
      } else if (status === "connecting") {
        setError(t("pairing.errorCannotConnect"));
        setStatus("idle");
      }
    };

    ws.onerror = () => {};
  }

  function startPolling(ws: WebSocket, sid: string) {
    const poll = () => {
      if (ws.readyState !== WebSocket.OPEN) return;
      const id = generateId();
      ws.send(
        JSON.stringify({
          type: "req",
          id,
          method: Methods.BROWSER_PAIRING_STATUS,
          params: { sender_id: sid },
        }),
      );
      pollRef.current = setTimeout(poll, 3000);
    };
    pollRef.current = setTimeout(poll, 3000);
  }

  function handleCancel() {
    if (pollRef.current) clearTimeout(pollRef.current);
    if (wsRef.current) wsRef.current.close();
    setCode(null);
    senderIDRef.current = null;
    setStatus("idle");
  }

  // Waiting for admin approval
  if (code && status === "pending") {
    return <PairingCodeDisplay code={code} onCancel={handleCancel} />;
  }

  // Approved
  if (status === "approved") {
    return (
      <p className="text-center text-sm text-success">
        {t("pairing.approved")}
      </p>
    );
  }

  // Request form
  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      {error && (
        <p className="text-center text-sm text-destructive">{error}</p>
      )}

      <div className="space-y-2">
        <Label htmlFor="pairingUserId">
          {t("pairing.userId")}
        </Label>
        <Input
          id="pairingUserId"
          type="text"
          value={userId}
          onChange={(e) => setUserId(e.target.value)}
          placeholder={t("pairing.userIdPlaceholder")}
          autoFocus
        />
      </div>

      <p className="text-xs text-muted-foreground">
        {t("pairing.noTokenNeeded")}
      </p>

      <Button
        type="submit"
        className="w-full"
        disabled={!userId.trim() || status === "connecting"}
      >
        {status === "connecting" ? t("pairing.connecting") : t("pairing.requestAccess")}
      </Button>
    </form>
  );
}

function PairingCodeDisplay({
  code,
  onCancel,
}: {
  code: string;
  onCancel: () => void;
}) {
  const { t } = useTranslation("login");
  return (
    <div className="space-y-4">
      <p className="text-center text-sm text-muted-foreground">
        {t("pairing.askAdmin")}
      </p>

      <div className="flex justify-center gap-1.5">
        {code.split("").map((char, i) => (
          <span
            key={i}
            className="flex h-10 w-8 items-center justify-center rounded-md border bg-muted font-mono text-lg font-bold"
          >
            {char}
          </span>
        ))}
      </div>

      <p className="text-center text-xs text-muted-foreground">
        {t("pairing.orRun")}{" "}
        <code className="rounded bg-muted px-1.5 py-0.5">
          argo pairing approve {code}
        </code>
      </p>

      <div className="flex items-center justify-center gap-2 text-sm text-muted-foreground">
        <span className="inline-block h-2 w-2 animate-pulse rounded-full bg-amber-500" />
        {t("pairing.waitingForApproval")}
      </div>

      <Button variant="outline" className="w-full" type="button" onClick={onCancel}>
        {t("pairing.cancel")}
      </Button>
    </div>
  );
}
