import {type ComponentProps} from 'react';
import {Button as ShadcnButton} from './components/ui/button';
import {Checkbox} from './components/ui/checkbox';
import {Popover, PopoverContent, PopoverTrigger} from './components/ui/popover';
import {ChevronDown, LoaderCircle} from 'lucide-react';
import type {tracker} from '../wailsjs/go/models';

export function RunningSpinner() {
    return <LoaderCircle aria-hidden="true" className="running-spinner"/>;
}

/** The project's icon, if it has one. */
export function ProjectIcon({project}: {project?: Pick<tracker.Project, 'icon'>}) {
    return project?.icon ? <img className="project-icon" src={project.icon} alt="" aria-hidden="true"/> : null;
}

/** The project's icon, or its color dot when it has no icon. */
export function ProjectMark({project, fallbackColor = '#94a3b8'}: {project?: Pick<tracker.Project, 'icon' | 'color'>; fallbackColor?: string}) {
    return project?.icon ? <ProjectIcon project={project}/> : <span className="dot" style={{background: project?.color ?? fallbackColor}}/>;
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

/** Parses a stored multiselect value (a JSON array of options); invalid values yield no selection. */
export function parseMultiselect(value: string | undefined): string[] {
    if (!value) return [];
    try {
        const parsed: unknown = JSON.parse(value);
        return Array.isArray(parsed) ? parsed.filter((item): item is string => typeof item === 'string') : [];
    } catch {
        return [];
    }
}

/** Dropdown that toggles any number of options; the selection keeps the option order. */
export function MultiSelect({label, options, selected, onChange}: {
    label: string; options: string[]; selected: string[]; onChange: (selected: string[]) => void;
}) {
    const toggle = (option: string, checked: boolean) =>
        onChange(options.filter(o => o === option ? checked : selected.includes(o)));
    return <Popover>
        <PopoverTrigger asChild>
            <ShadcnButton type="button" variant="outline" className="multi-select-trigger" aria-label={`${label}: ${selected.length ? selected.join(', ') : 'Not set'}`}>
                <span>{selected.length ? selected.join(', ') : 'Not set'}</span><ChevronDown aria-hidden="true"/>
            </ShadcnButton>
        </PopoverTrigger>
        <PopoverContent align="start" className="multi-select-content" role="group" aria-label={`${label} options`}>
            {options.map(option => <label className="check" key={option}>
                <Check checked={selected.includes(option)} onCheckedChange={checked => toggle(option, checked)}/>{option}
            </label>)}
            {selected.length > 0 && <ShadcnButton type="button" variant="ghost" size="sm" onClick={() => onChange([])}>Clear selection</ShadcnButton>}
        </PopoverContent>
    </Popover>;
}
