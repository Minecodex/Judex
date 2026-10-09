import {createContext, useContext, type ComponentProps} from 'react';

const ActionSize = createContext<'sm' | 'md' | 'lg' | undefined>(undefined);
export const useActionSize = () => useContext(ActionSize);

export function ActionGroup({children,className='',size='md',...props}:ComponentProps<'div'> & {size?:'sm'|'md'|'lg'}) {
  return <ActionSize.Provider value={size}><div {...props} className={'judex-action-group '+className}>{children}</div></ActionSize.Provider>;
}

export function CardActions({children, className = '',...props}: ComponentProps<'div'>) {
  return <ActionGroup {...props} size="sm" className={'judex-card-actions '+className}>{children}</ActionGroup>;
}
