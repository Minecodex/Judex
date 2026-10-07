import {Button as HeroButton} from '@heroui/react';
import type {ComponentProps} from 'react';

// Keep the domain components independent from a UI-library-specific event shape.
export function Button({disabled,type='button',className='',title,variant='ghost',...props}:Omit<ComponentProps<typeof HeroButton>,'isDisabled'> & {disabled?:boolean;title?:string}){
 return <HeroButton {...props} aria-label={props['aria-label']??title} type={type} isDisabled={disabled} variant={variant} className={'judex-control '+className}/>;
}
