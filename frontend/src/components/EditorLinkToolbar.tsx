/**
 * The toolbar BlockNote floats over a hovered (or caret-selected) link, plus two
 * copy actions this app needs: the anchor's text and its target URL.
 *
 * BlockNote's default toolbar only offers edit / open / unlink, and its
 * "open" is a left click away from navigating — copying either half of a link
 * meant selecting it and using Ctrl+C, or editing it to read the URL. The two
 * buttons here write straight to the system clipboard; the icon flips to a
 * check for a moment so the action is visibly done.
 *
 * The default buttons are re-composed rather than re-implemented, so the edit
 * popover and the unlink behaviour stay BlockNote's own.
 */
import { useState } from "react";
import type { ReactNode } from "react";
import {
  DeleteLinkButton,
  EditLinkButton,
  LinkToolbar,
  OpenLinkButton,
  useComponentsContext,
} from "@blocknote/react";
import type { LinkToolbarProps } from "@blocknote/react";
import { Check, Link2, Type } from "lucide-react";
import { writeClipboardText } from "../services/clipboard";
import { t } from "../services/i18n";

/** One copy action: writes `value` and acknowledges with a check. */
function CopyLinkToolbarButton({
  icon,
  label,
  value,
}: {
  icon: ReactNode;
  label: string;
  value: string;
}) {
  const Components = useComponentsContext();
  const [copied, setCopied] = useState(false);
  if (!Components) return null;

  return (
    <Components.LinkToolbar.Button
      className="bn-button"
      label={label}
      mainTooltip={label}
      icon={copied ? <Check size={16} /> : icon}
      onClick={() => {
        void writeClipboardText(value).then((written) => {
          if (!written) return;
          setCopied(true);
          window.setTimeout(() => setCopied(false), 1200);
        });
      }}
    />
  );
}

/** Drop-in replacement for BlockNote's default link toolbar. */
export function SliteLinkToolbar(props: LinkToolbarProps) {
  return (
    <LinkToolbar {...props}>
      <EditLinkButton {...props} />
      <CopyLinkToolbarButton
        icon={<Link2 size={16} />}
        label={t.linkCopyLink}
        value={props.url}
      />
      <CopyLinkToolbarButton
        icon={<Type size={16} />}
        label={t.linkCopyText}
        value={props.text}
      />
      <OpenLinkButton url={props.url} />
      <DeleteLinkButton
        range={props.range}
        setToolbarOpen={props.setToolbarOpen}
      />
    </LinkToolbar>
  );
}
