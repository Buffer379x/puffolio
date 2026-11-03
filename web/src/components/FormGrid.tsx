import { Children, Fragment, isValidElement, type ReactNode } from 'react';
import { formSpans } from './formLayout';
function flatten(children: ReactNode): ReactNode[] {
  return Children.toArray(children).flatMap((child) =>
    isValidElement<{ children?: ReactNode }>(child) && child.type === Fragment
      ? flatten(child.props.children)
      : [child],
  );
}
export function FormGrid({ children }: { children: ReactNode }) {
  const items = flatten(children);
  const full = formSpans(
    items.map((item) => {
      if (!isValidElement<{ wide?: boolean; className?: string; field?: { type: string } }>(item))
        return false;
      const p = item.props;
      return (
        !!p.wide ||
        !!p.className?.split(' ').includes('full') ||
        (!!p.field && ['rating', 'image', 'tags', 'longtext'].includes(p.field.type))
      );
    }),
  );
  return (
    <div className="form-grid">
      {items.map((child, i) => (
        <div
          className={'form-grid-cell ' + (full[i] ? 'full' : '')}
          key={isValidElement(child) ? (child.key ?? i) : i}
        >
          {child}
        </div>
      ))}
    </div>
  );
}
