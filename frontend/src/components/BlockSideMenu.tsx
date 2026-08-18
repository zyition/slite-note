import { useEffect, useState } from "react";
import type { FC } from "react";
import { SideMenuExtension } from "@blocknote/core/extensions";
import {
  DragHandleButton,
  useExtensionState,
  type SideMenuProps,
} from "@blocknote/react";

/**
 * BlockSideMenu — the per-block drag handle shown on hover, aligned to the
 * first line of the block.
 *
 * BlockNote computes the default handle's vertical offset from its *default*
 * heading sizes (3em/2em/1.3em); since slite uses much smaller headings, the
 * handle would sit below the focused line. We measure the actual first line
 * center of the block and counter-shift with a translateY so the handle is
 * centered on the first line for every block type.
 */

const MENU_H = 24;

// BlockNote's hardcoded cross-axis offsets for the handle (ported from
// SideMenuController.getBlockOffset), keyed by heading level.
const BN_HEADING_OFFSET: Record<number, number> = { 1: 39, 2: 27, 3: 18.5 };

export const BlockSideMenu: FC<SideMenuProps> = (props) => {
  const block = useExtensionState(SideMenuExtension, {
    selector: (s) => s?.block,
  });

  const [shift, setShift] = useState(0);

  // Re-measure whenever the target block changes. BlockNote offsets the
  // handle from the block's top (crossAxis offset), so we compute the desired
  // offset as (block top → first line center) − handle height/2.
  useEffect(() => {
    if (!block) {
      setShift(0);
      return;
    }
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

  return (
    <div className="slite-side-menu" style={{ transform: `translateY(${shift.toFixed(1)}px)` }}>
      <DragHandleButton dragHandleMenu={props.dragHandleMenu} />
    </div>
  );
};
