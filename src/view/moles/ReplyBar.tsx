import type { KeyboardEvent } from "react";
import { Box, Button, IconButton, InputAdornment, Stack, TextField, Tooltip } from "@mui/material";
import ReplyAllIcon from "@mui/icons-material/ReplyAll";
import EditIcon from "@mui/icons-material/Edit";
import SendIcon from "@mui/icons-material/Send";

interface ReplyBarProps {
  value: string;
  onChange: (value: string) => void;
  onSend: () => void;
  onCustomize: () => void;
  onReplyAll: () => void;
  /** Whether more than one participant makes "Reply all" meaningful. */
  canReplyAll?: boolean;
  placeholder?: string;
  sending?: boolean;
}

// A real text input for a quick plain-text reply, sent inline on Enter/Send.
// The pen icon beside it opens the full compose popup (recipients, subject,
// templates, attachments) prefilled with whatever's been typed so far.
const ReplyBar = ({
  value,
  onChange,
  onSend,
  onCustomize,
  onReplyAll,
  canReplyAll = true,
  placeholder = "Reply to this conversation…",
  sending = false,
}: ReplyBarProps) => {
  const canSend = value.trim().length > 0 && !sending;

  const handleKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      if (canSend) onSend();
    }
  };

  return (
    <Box
      sx={{
        p: 2,
        borderTop: 1,
        borderColor: "divider",
        backgroundColor: "background.paper",
        display: "flex",
        flexDirection: { xs: "column", sm: "row" },
        alignItems: { xs: "stretch", sm: "flex-end" },
        gap: 1.5,
      }}
    >
      <TextField
        fullWidth
        multiline
        maxRows={6}
        size="small"
        placeholder={placeholder}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={handleKeyDown}
        disabled={sending}
        sx={{
          flex: 1,
          "& .MuiOutlinedInput-root": { borderRadius: 3 },
        }}
        slotProps={{
          input: {
            endAdornment: (
              <InputAdornment position="end" sx={{ alignItems: "flex-start", mt: 0.5 }}>
                <Tooltip title="Customize (recipients, subject, formatting…)">
                  <IconButton size="small" onClick={onCustomize} aria-label="Customize reply">
                    <EditIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
                <Tooltip title="Send">
                  <span>
                    <IconButton
                      size="small"
                      color="primary"
                      onClick={onSend}
                      disabled={!canSend}
                      aria-label="Send reply"
                    >
                      <SendIcon fontSize="small" />
                    </IconButton>
                  </span>
                </Tooltip>
              </InputAdornment>
            ),
          },
        }}
      />
      {canReplyAll && (
        <Stack direction="row" spacing={1} sx={{ flexShrink: 0 }}>
          <Button variant="outlined" startIcon={<ReplyAllIcon />} onClick={onReplyAll}>
            Reply all
          </Button>
        </Stack>
      )}
    </Box>
  );
};

export default ReplyBar;
