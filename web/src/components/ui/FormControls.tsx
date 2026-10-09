import {
  Children,
  createContext,
  useContext,
  isValidElement,
  useState,
  useId,
  type ReactNode,
  type ComponentProps,
} from "react";
import {
  Input,
  TextArea,
  Select,
  ListBox,
  Label,
  Checkbox,
  Card,
  Chip,
  Alert,
  Disclosure,
} from "@heroui/react";
export { FilePicker as UIFilePicker } from "./FilePicker";
const FieldLabel = createContext<{ label: string; id: string } | undefined>(
  undefined,
);
export function FormField({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  const id = useId();
  return (
    <FieldLabel.Provider value={{ label, id }}>
      <div className="judex-field-label">
        <Label htmlFor={id}>{label}</Label>
        {children}
      </div>
    </FieldLabel.Provider>
  );
}
export function UIInput(props: ComponentProps<typeof Input>) {
  const field = useContext(FieldLabel);
  return (
    <Input
      {...props}
      id={props.id ?? field?.id}
      aria-label={props["aria-label"] ?? field?.label}
    />
  );
}
export function UITextArea(props: ComponentProps<typeof TextArea>) {
  const field = useContext(FieldLabel);
  return (
    <TextArea
      {...props}
      id={props.id ?? field?.id}
      aria-label={props["aria-label"] ?? field?.label}
    />
  );
}
type ChoiceEvent = {
  target: { value: string };
  currentTarget: { value: string };
};
type OptionProps = {
  value?: string | number;
  disabled?: boolean;
  children?: ReactNode;
};
export function UIOption(_: OptionProps) {
  return null;
}
function plainText(node: ReactNode): string {
  return Children.toArray(node)
    .map((child) =>
      typeof child === "string" || typeof child === "number"
        ? String(child)
        : isValidElement<{ children?: ReactNode }>(child)
          ? plainText(child.props.children)
          : "",
    )
    .join("");
}
export function UISelect({
  value,
  defaultValue,
  onChange,
  children,
  className,
  disabled,
  "aria-label": ariaLabel,
  "data-testid": testId,
  id,
  name,
}: {
  value?: string | number;
  defaultValue?: string | number;
  onChange?: (event: ChoiceEvent) => void;
  children: ReactNode;
  className?: string;
  disabled?: boolean;
  "aria-label"?: string;
  "data-testid"?: string;
  id?: string;
  name?: string;
}) {
  const field = useContext(FieldLabel),
    label = field?.label;
  const options: OptionProps[] = [];
  const collect = (nodes: ReactNode) =>
    Children.forEach(nodes, (child) => {
      if (!isValidElement<OptionProps>(child)) return;
      if (child.type === UIOption) options.push(child.props);
      else collect(child.props.children);
    });
  collect(children);
  const [local, setLocal] = useState(
    String(defaultValue ?? options[0]?.value ?? ""),
  );
  const selected = String(value ?? local),
    encode = (v: string) => "choice:" + v;
  return (
    <Select
      className="judex-ui-select"
      aria-label={ariaLabel ?? label}
      name={name}
      isDisabled={disabled}
      value={encode(selected)}
      onChange={(key) => {
        if (key == null) return;
        const next = String(key).slice(7);
        setLocal(next);
        onChange?.({ target: { value: next }, currentTarget: { value: next } });
      }}
    >
      <Select.Trigger
        id={id ?? field?.id}
        className={className ?? "judex-input"}
        data-testid={testId}
        data-value={selected}
      >
        <Select.Value />
        <Select.Indicator />
      </Select.Trigger>
      <Select.Popover className="judex-ui-select-popover">
        <ListBox aria-label={ariaLabel ?? label}>
          {options.map((option) => {
            const text = plainText(option.children),
              v = String(option.value ?? text);
            return (
              <ListBox.Item
                key={encode(v)}
                id={encode(v)}
                textValue={text}
                isDisabled={option.disabled}
                data-option-value={v}
              >
                <Label>{option.children}</Label>
                <ListBox.ItemIndicator />
              </ListBox.Item>
            );
          })}
        </ListBox>
      </Select.Popover>
    </Select>
  );
}
type CheckEvent = {
  target: { checked: boolean };
  currentTarget: { checked: boolean };
};
export function UICheckbox({
  checked,
  onChange,
  children,
  className,
  disabled,
  "data-testid": testId,
  "aria-label": label,
  appearance = 'plain',
}: {
  checked?: boolean;
  onChange?: (event: CheckEvent) => void;
  children?: ReactNode;
  className?: string;
  disabled?: boolean;
  "data-testid"?: string;
  "aria-label"?: string;
  appearance?: 'plain' | 'card';
}) {
  return (
    <Checkbox
      className={"judex-checkbox-field " + (appearance === 'card' ? 'judex-checkbox-card ' : '') + (className ?? "")}
      isSelected={checked}
      isDisabled={disabled}
      onChange={(selected) =>
        onChange?.({
          target: { checked: selected },
          currentTarget: { checked: selected },
        })
      }
    >
      <Checkbox.Content data-testid={testId} aria-label={label}>
        <Checkbox.Control>
          <Checkbox.Indicator />
        </Checkbox.Control>
        <Label className="judex-checkbox-label">{children}</Label>
      </Checkbox.Content>
    </Checkbox>
  );
}
export function UICard({
  className = "",
  ...props
}: ComponentProps<typeof Card>) {
  return <Card {...props} className={"judex-ui-card " + className} />;
}
export function UIStatus({
  className = "",
  children,
  ...props
}: ComponentProps<typeof Chip>) {
  return (
    <Chip {...props} className={'judex-ui-status '+(!className && !props.color ? 'judex-ui-status-neutral ' : '')+className}>
      <Chip.Label>{children}</Chip.Label>
    </Chip>
  );
}
export function UIWarning({
  className = "",
  children,
  ...props
}: ComponentProps<typeof Alert>) {
  return (
    <Alert
      status="warning"
      {...props}
      className={"judex-ui-alert " + className}
    >
      <Alert.Content>
        <Alert.Description>{children}</Alert.Description>
      </Alert.Content>
    </Alert>
  );
}
export function UINotice({
  className = "",
  children,
  ...props
}: ComponentProps<typeof Alert>) {
  return (
    <Alert {...props} className={"judex-ui-notice " + className}>
      {children}
    </Alert>
  );
}
export function UIDisclosure({
  title,
  children,
  className = "",
  open = false,
  isExpanded,
  onExpandedChange,
}: {
  title: ReactNode;
  children?: ReactNode;
  className?: string;
  open?: boolean;
  isExpanded?: boolean;
  onExpandedChange?: (expanded:boolean)=>void;
}) {
  return (
    <Disclosure
      defaultExpanded={open}
      isExpanded={isExpanded}
      onExpandedChange={onExpandedChange}
      className={"judex-ui-disclosure " + className}
    >
      <Disclosure.Heading>
        <Disclosure.Trigger>
          {title}
          <Disclosure.Indicator />
        </Disclosure.Trigger>
      </Disclosure.Heading>
      <Disclosure.Content>
        <Disclosure.Body>{children}</Disclosure.Body>
      </Disclosure.Content>
    </Disclosure>
  );
}
