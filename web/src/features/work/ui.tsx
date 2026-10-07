import {
  FormField,
  UIStatus,
  UIFilePicker,
} from "../../components/ui/FormControls";
import { Button as HeroButton, Modal } from "@heroui/react";
import { Button } from "../../components/ui/Button";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { X, ArrowUpRight, Paperclip, FileText, Check } from "lucide-react";
import { useWork } from "./store";
import { dataMode } from "../../lib/api/client";
import type { Evidence, Task, Source, Plan } from "./types";
import { uid } from "./seed";
export function Btn({
  children,
  onClick,
  secondary = false,
  disabled = false,
  testId,
  danger = false,
}: {
  children: ReactNode;
  onClick?: () => void;
  secondary?: boolean;
  disabled?: boolean;
  testId?: string;
  danger?: boolean;
}) {
  return (
    <HeroButton
      variant={danger ? "danger" : secondary ? "secondary" : "primary"}
      isDisabled={disabled}
      onPress={onClick}
      data-testid={testId}
      className={
        "judex-button judex-button-" +
        (danger ? "danger" : secondary ? "secondary" : "primary")
      }
    >
      {children}
    </HeroButton>
  );
}
export function Dialog({
  title,
  onClose,
  children,
  wide = false,
}: {
  title: string;
  onClose: () => void;
  children: ReactNode;
  wide?: boolean;
}) {
  const { t, theme } = useWork();
  return (
    <Modal.Backdrop
      isOpen
      isDismissable
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      className="judex-overlay"
      data-theme={theme}
    >
      <Modal.Container
        size={wide ? "lg" : "md"}
        className={
          "judex-dialog-container" +
          (wide ? " judex-dialog-container-wide" : "")
        }
      >
        <Modal.Dialog aria-label={title} className="judex-dialog-content">
          <Modal.Header className="judex-dialog-header">
            <Modal.Heading>{title}</Modal.Heading>
            <HeroButton
              className="judex-dialog-close"
              variant="ghost"
              isIconOnly
              aria-label={t("close")}
              onPress={onClose}
            >
              <X />
            </HeroButton>
          </Modal.Header>
          <Modal.Body className="judex-dialog-body">{children}</Modal.Body>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}
export function Person({
  seatId,
  name,
  small = false,
}: {
  seatId?: string;
  name?: string;
  small?: boolean;
}) {
  const { state, text } = useWork(),
    seat = state.seats.find((s) => s.id === seatId),
    pos = state.positions.find((p) => p.id === seat?.positionId),
    person = name ?? seat?.person ?? "Judex";
  return (
    <span
      className={
        "judex-work-person" + (small ? " judex-work-person-small" : "")
      }
    >
      <span className={"judex-initial judex-tone-" + (pos?.tone ?? "mint")}>
        {person.slice(0, 1)}
      </span>
      <span>
        <strong>{person}</strong>
        {!small && pos && <small>{text(pos.name)}</small>}
      </span>
    </span>
  );
}
export function Pill({
  status,
  kind = "task",
}: {
  status: Task["status"] | Source["status"] | Plan["status"] | "blocked";
  kind?: "task" | "source" | "plan";
}) {
  const { t } = useWork();
  return (
    <UIStatus className={"judex-work-pill judex-work-pill-" + status}>
      {status === "accepted"
        ? t(
            kind === "task"
              ? "workTaskAccepted"
              : kind === "plan"
                ? "workPlanCompleted"
                : "workSourceReceived",
          )
        : t(
            ("workStatus" +
              status[0].toUpperCase() +
              status.slice(1)) as Parameters<typeof t>[0],
          )}
    </UIStatus>
  );
}
export function Heading({
  eyebrow,
  title,
  description,
  children,
}: {
  eyebrow?: string;
  title: string;
  description?: string;
  children?: ReactNode;
}) {
  return (
    <div className="judex-work-heading-row">
      <div>
        {eyebrow && <span className="judex-eyebrow">{eyebrow}</span>}
        <h1>{title}</h1>
        {description && <p>{description}</p>}
      </div>
      {children && <div className="judex-work-heading-actions">{children}</div>}
    </div>
  );
}
export function Field({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return <FormField label={label}>{children}</FormField>;
}
export function EmptyState({
  text,
  children,
}: {
  text: string;
  children?: ReactNode;
}) {
  return (
    <div className="judex-work-empty">
      <Check />
      <p>{text}</p>
      {children}
    </div>
  );
}
export function TaskRow({
  task,
  nested = false,
}: {
  task: Task;
  nested?: boolean;
}) {
  const { go, text, t, state } = useWork(),
    plan = state.plans.find((p) => p.id === task.planId);
  return (
    <Button
      data-testid={"work-task-" + task.id}
      className={
        "judex-work-task-row" + (nested ? " judex-work-task-nested" : "")
      }
      onClick={() =>
        go({
          view: "task",
          id: task.id,
        })
      }
    >
      <span
        className={
          "judex-task-check " +
          (task.status === "accepted" ? "judex-task-check-done" : "")
        }
      >
        {task.status === "accepted" && <Check />}
      </span>
      <span className="judex-work-task-name">
        <strong>{text(task.title)}</strong>
        <small>{plan ? text(plan.title) : t("workDirect")}</small>
      </span>
      <Pill status={task.status} />
      <Person seatId={task.seatIds[0]} small />
      <ArrowUpRight />
    </Button>
  );
}
export function EvidenceList({ files }: { files: Evidence[] }) {
  const [selected, setSelected] = useState<Evidence | null>(null),
    { t } = useWork();
  return (
    <>
      <div className="judex-work-files">
        {files.map((file) => (
          <Button key={file.id} onClick={() => setSelected(file)}>
            <FileText />
            <span>
              {file.name}
              <small>{file.author}</small>
            </span>
            <ArrowUpRight />
          </Button>
        ))}
      </div>
      {selected && (
        <Dialog title={selected.name} onClose={() => setSelected(null)} wide>
          <p className="judex-muted">
            {selected.author} · {new Date(selected.at).toLocaleString()}
          </p>
          {selected.type?.startsWith("image/") && selected.data ? (
            <img
              className="judex-work-evidence-image"
              src={selected.data}
              alt={selected.name}
            />
          ) : (
            <pre className="judex-work-report">
              {selected.text || t("workBinary")}
            </pre>
          )}
          {selected.data && (
            <a
              className="judex-work-download"
              href={selected.data}
              download={selected.name}
            >
              {t("workDownload")}
            </a>
          )}
        </Dialog>
      )}
    </>
  );
}
export function Upload({
  files,
  onChange,
}: {
  files: Evidence[];
  onChange: (files: Evidence[]) => void;
}) {
  const { state, t, setToast } = useWork();
  return (
    <div className="judex-work-upload">
      <UIFilePicker
        multiple
        data-testid="work-files"
        onChange={async (e) => {
          const values = Array.from(e.target.files ?? []),
            incoming: Evidence[] = [];
          // demo 模式存 localStorage，限 1MB；api 模式走分片上传，上限由服务端声明。
          const cap = dataMode === "demo" ? 1024 * 1024 : 100 * 1024 * 1024;
          for (const file of values) {
            if (file.size > cap) {
              setToast(t("workFileLarge"));
              continue;
            }
            const textual =
              file.type.startsWith("text/") ||
              /\.(md|txt|json|csv|log)$/i.test(file.name);
            const data = await new Promise<string>((resolve, reject) => {
              const reader = new FileReader();
              reader.onload = () => resolve(String(reader.result));
              reader.onerror = reject;
              reader.readAsDataURL(file);
            });
            incoming.push({
              id: uid(),
              name: file.name,
              text: textual ? await file.text() : "",
              data,
              type: file.type,
              author: state.currentUser,
              at: Date.now(),
            });
          }
          onChange([...files, ...incoming]);
          e.target.value = "";
        }}
      >
        <Paperclip />
        {t("workAttach")}
      </UIFilePicker>
      {files.map((f) => (
        <span key={f.id}>
          {f.name}
          <Button
            aria-label={t("workRemove")}
            onClick={() => onChange(files.filter((v) => v.id !== f.id))}
          >
            <X />
          </Button>
        </span>
      ))}
    </div>
  );
}
