import { useRef, type InputHTMLAttributes, type ReactNode } from "react";
import { Button } from "./Button";
export function FilePicker({
  children,
  ...props
}: Omit<InputHTMLAttributes<HTMLInputElement>, "type" | "children"> & {
  children: ReactNode;
}) {
  const input = useRef<HTMLInputElement>(null);
  return (
    <div className="judex-file-picker">
      <Button variant="secondary" onPress={() => input.current?.click()}>
        {children}
      </Button>
      {/* The browser file API requires a native input. It is hidden; HeroUI owns the visible control. */}
      <input {...props} ref={input} type="file" hidden />
    </div>
  );
}
