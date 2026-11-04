import {type ComponentProps} from 'react';
import {Button as ShadcnButton} from './components/ui/button';
import {Checkbox} from './components/ui/checkbox';
import {LoaderCircle} from 'lucide-react';

export function RunningSpinner() {
    return <LoaderCircle aria-hidden="true" className="running-spinner"/>;
}

export const BUSY_REASON = 'Please wait until the current action finishes.';

/** Returns the message of the first check whose condition is true. */
export function whyDisabled(...checks: [boolean, string][]): string | undefined {
    return checks.find(([condition]) => condition)?.[1];
}

/** A `disabledReason` disables the button and explains why on hover. */
export function Button({className = '', variant, disabled, disabledReason, title, ...props}: ComponentProps<typeof ShadcnButton> & {disabledReason?: string}) {
    const isDisabled = disabled === true || disabledReason !== undefined;
    return <ShadcnButton className={className} variant={variant ?? (className.includes('primary') ? 'default' : className.includes('danger') ? 'destructive' : 'outline')}
        disabled={isDisabled} title={isDisabled && disabledReason ? disabledReason : title} {...props}/>;
}

export function Check({checked, onCheckedChange, ...props}: Omit<ComponentProps<typeof Checkbox>, 'checked' | 'onCheckedChange'> & {
    checked: boolean; onCheckedChange: (checked: boolean) => void;
}) {
    return <Checkbox checked={checked} onCheckedChange={value => onCheckedChange(value === true)} {...props}/>;
}
