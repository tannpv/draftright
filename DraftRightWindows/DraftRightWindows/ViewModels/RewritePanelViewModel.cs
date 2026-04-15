using System;
using System.Collections.ObjectModel;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using DraftRightWindows.Models;
using DraftRightWindows.Services;
using Windows.ApplicationModel.DataTransfer;

namespace DraftRightWindows.ViewModels;

public partial class RewritePanelViewModel : ObservableObject
{
    // ── Observable properties ──

    [ObservableProperty]
    private string _inputText = string.Empty;

    [ObservableProperty]
    private string _outputText = string.Empty;

    [ObservableProperty]
    private Tone? _selectedTone;

    [ObservableProperty]
    private bool _isLoading;

    [ObservableProperty]
    private string _errorMessage = string.Empty;

    [ObservableProperty]
    private string _usageInfo = string.Empty;

    // ── Collections ──

    public ObservableCollection<Tone> Tones { get; } = new(
        (Tone[])Enum.GetValues(typeof(Tone))
    );

    // ── Events ──

    /// <summary>Raised when the panel should close.</summary>
    public event EventHandler? CloseRequested;

    // ── Commands ──

    [RelayCommand]
    private async Task RewriteAsync(Tone tone)
    {
        if (string.IsNullOrWhiteSpace(InputText))
        {
            ErrorMessage = "No text selected.";
            return;
        }

        SelectedTone = tone;
        ErrorMessage = string.Empty;
        OutputText = string.Empty;
        IsLoading = true;

        try
        {
            var targetLanguage = tone == Tone.Translate
                ? App.Settings.TranslateLanguage
                : null;

            var result = await App.Api.RewriteAsync(
                InputText,
                tone.ApiValue(),
                targetLanguage);

            OutputText = result.RewrittenText;
            UsageInfo = $"{result.UsageToday} / {result.DailyLimit} rewrites today";
        }
        catch (ApiException ex)
        {
            ErrorMessage = ex.Message;
        }
        catch (Exception ex)
        {
            ErrorMessage = $"Rewrite failed: {ex.Message}";
        }
        finally
        {
            IsLoading = false;
        }
    }

    [RelayCommand]
    private void Replace()
    {
        if (string.IsNullOrWhiteSpace(OutputText))
            return;

        // Copy to clipboard then simulate paste (Ctrl+V) via the clipboard service
        var dataPackage = new DataPackage();
        dataPackage.SetText(OutputText);
        Clipboard.SetContent(dataPackage);

        CloseRequested?.Invoke(this, EventArgs.Empty);
    }

    [RelayCommand]
    private void Copy()
    {
        if (string.IsNullOrWhiteSpace(OutputText))
            return;

        var dataPackage = new DataPackage();
        dataPackage.SetText(OutputText);
        Clipboard.SetContent(dataPackage);
    }

    [RelayCommand]
    private void Close()
    {
        CloseRequested?.Invoke(this, EventArgs.Empty);
    }
}
