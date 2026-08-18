import { useEffect, useState } from "react";
import type { FC } from "react";
import { SideMenuExtension } from "@blocknote/core/extensions";
import type { Block } from "@blocknote/core";
import {
  DragHandleButton,
  useExtensionState,
  type SideMenuProps,
} from "@blocknote/react";

/**
 * BlockSideMenu — the per-block handle shown on hover, Feishu-style:
 * a small badge with the block's format hint (H1 / H2 / ☐ / • / ❝ / { }) next
 * to the drag handle.
 *
 * BlockNote computes the default handle's vertical offset from its *default*
 * heading sizes (3em/2em/1.3em); since slite uses much smaller headings, the
 * handle would sit below the focused line. We measure the actual first line
 * height of the block and counter-shift with a translateY.
 */

const BADGES: Record<string, (b: Block<any, any, any>) => string> = {
  heading: (b) => `H${(b.props.level as number) ?? 1}`,
  checkListItem: (b) => (b.props.checked ? "☑" : "☐"),
  bulletListItem: () => "•",
  numberedListItem: () => "1",
  quote: () => "❝",
  codeBlock: () => "{ }",
  divider: () => "—",
  table: () => "⊞",
  paragraph: () => "",
};

// BlockNote's hardcoded cross-axis offsets for the handle (ported from
// SideMenuController.getBlockOffset), keyed by heading level.
const BN_HEADING_OFFSET: Record<number, number> = { 1: 39, 2: 27, 3: 18.5 };

const MENU_H = 30;

function badgeFor(block: Block<any, any, any> | undefined): string {
  if (!block) return "";
  const fn = BADGES[block.type];
  return fn ? fn(block) : "";
}

export const BlockSideMenu: FC<SideMenuProps> = (props) => {
  const block = useExtensionState(SideMenuExtension, {
    selector: (s) => s?.block,
  });

  const [shift, setShift] = useState(0);

  // Re-measure whenever the target block changes. BlockNote offsets the
  // handle from the block's top (crossAxis offset), so we compute the desired
  // offset as (block top → first line center) − menu height/2.
  useEffect(() => {
    if (!block) return;
    const el = document.querySelector(`.bn-block[data-id="${block.id}"]`);
    const line = el?.querySelector(".bn-inline-content");
    if (!el || !line) {
      setShift(0);
      return;
    }
    const blockRect = el.getBoundingClientRect();
    const lineRect = line.getBoundingClientRect();
    const lineCenter = lineRect.top + lineRect.height / 2;
    const desired = lineCenter - blockRect.top - MENU_H / 2;
    const bnOffset =
      block.type === "heading"
        ? (BN_HEADING_OFFSET[(block.props.level as number) ?? 1] ?? 0)
        : 0;
    setShift(desired - bnOffset);
  }, [block]);

  const badge = badgeFor(block);

  return (
    <div
      className="slite-side-menu"
      style={{ transform: `translateY(${shift.toFixed(1)}px)` }}
    >
      {badge && <span className="slite-side-menu-badge">{badge}</span>}
      <DragHandleButton dragHandleMenu={props.dragHandleMenu} />
    </div>
  );
};
