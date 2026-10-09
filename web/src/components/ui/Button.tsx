import {Button as HeroButton} from '@heroui/react';
import type {ComponentProps} from 'react';
import {useActionSize} from './ActionGroup';

// Keep the domain components independent from a UI-library-specific event shape.
export function Button({disabled,type='button',className='',title,variant='ghost',size,...props}:ComponentProps<typeof HeroButton> & {disabled?:boolean;title?:string}){
 const contextualSize = useActionSize();
 return <HeroButton {...props} size={size??contextualSize??'md'} aria-label={props['aria-label']??title} type={type} isDisabled={disabled??props.isDisabled} variant={variant} className={'judex-control '+className}/>;
}
