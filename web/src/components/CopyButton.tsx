import { ReactNode, useEffect, useRef, useState } from "react";

import { Check, ContentCopy } from "@mui/icons-material";
import { Button, CircularProgress, SxProps, Tooltip } from "@mui/material";

export interface Props {
    variant?: "contained" | "outlined" | "text";
    tooltip: string;
    children: ReactNode;
    childrenCopied?: ReactNode;
    value: null | string;
    msTimeoutCopying?: number;
    msTimeoutCopied?: number;
    sx?: SxProps;
    fullWidth?: boolean;
}

const msTimeoutDefaultCopying = 500;
const msTimeoutDefaultCopied = 2000;

const CopyButton = function (props: Props) {
    const [isCopied, setIsCopied] = useState(false);
    const [isCopying, setIsCopying] = useState(false);
    const mountedRef = useRef(true);
    const copyingTimeoutRef = useRef<ReturnType<typeof setTimeout>>(undefined);
    const copiedTimeoutRef = useRef<ReturnType<typeof setTimeout>>(undefined);
    const msTimeoutCopying = props.msTimeoutCopying ?? msTimeoutDefaultCopying;
    const msTimeoutCopied = props.msTimeoutCopied ?? msTimeoutDefaultCopied;

    useEffect(
        () => () => {
            mountedRef.current = false;
            clearTimeout(copyingTimeoutRef.current);
            clearTimeout(copiedTimeoutRef.current);
        },
        [],
    );

    const handleCopyToClipboard = () => {
        if (isCopied || !props.value || props.value === "") {
            return;
        }

        (async (value: string) => {
            setIsCopying(true);

            await navigator.clipboard.writeText(value);

            if (!mountedRef.current) {
                return;
            }

            copyingTimeoutRef.current = setTimeout(() => {
                setIsCopying(false);
                setIsCopied(true);
            }, msTimeoutCopying);

            copiedTimeoutRef.current = setTimeout(() => {
                setIsCopied(false);
            }, msTimeoutCopied);
        })(props.value);
    };

    const variant = props.variant ?? "outlined";
    const color = isCopied ? "success" : "primary";
    const displayText = isCopied && props.childrenCopied ? props.childrenCopied : props.children;

    let icon;

    if (isCopying) {
        icon = <CircularProgress color="inherit" size={20} />;
    } else if (isCopied) {
        icon = <Check />;
    } else {
        icon = <ContentCopy />;
    }

    return props.value === null || props.value === "" ? (
        <Button variant={variant} color={color} disabled sx={props.sx} fullWidth={props.fullWidth} startIcon={icon}>
            {displayText}
        </Button>
    ) : (
        <Tooltip title={props.tooltip}>
            <Button
                variant={variant}
                color={color}
                onClick={isCopying ? undefined : handleCopyToClipboard}
                sx={props.sx}
                fullWidth={props.fullWidth}
                startIcon={icon}
            >
                {displayText}
            </Button>
        </Tooltip>
    );
};

export default CopyButton;
