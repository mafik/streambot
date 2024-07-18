#pragma once

#define TGI_VERSION_MAJOR 9
#define TGI_VERSION_MINOR 0
#define TGI_VERSION_REVISION 4

#ifdef _WIN64
#ifdef _DEBUG
    const char g_DllName[] = "tobii_gameintegration_x64_d.dll";
#else
    const char g_DllName[] = "tobii_gameintegration_x64.dll";
#endif
#else
#ifdef _DEBUG
    const char g_DllName[] = "tobii_gameintegration_x86_d.dll";
#else
    const char g_DllName[] = "tobii_gameintegration_x86.dll";
#endif
#endif

#if defined(GAME_INTEGRATION_EXPORT)
#   define TGI_TYPE_EXPORT __declspec(dllexport)
#   define TGI_C_EXPORT extern "C" __declspec(dllexport)
#else
#   define TGI_TYPE_EXPORT
#   define TGI_C_EXPORT
#endif

#include <cstdint>
#include <assert.h>

namespace TobiiGameIntegration
{
    struct ITobiiGameIntegrationApi;

#ifdef __cplusplus
    extern "C"
    {
#endif
        // Please, specify a full, real game name, not a code name for the game.
        // Example: "Assassin's Creed Origins"

        TGI_C_EXPORT ITobiiGameIntegrationApi* __cdecl GetApi(const char* fullGameName, int majorVersion, int minorVersion, int revision, const uint16_t* license, uint32_t licenseSize, bool analyticalUse);
        TGI_C_EXPORT ITobiiGameIntegrationApi* __cdecl GetApiDynamic(const char* fullGameName, const char* dllPath, int majorVersion, int minorVersion, int revision, const uint16_t* license, uint32_t licenseSize, bool analyticalUse);

#ifdef __cplusplus
    }
#endif

    inline ITobiiGameIntegrationApi* GetApi(const char* fullGameName, bool analyticalUse = false)
    {
        return GetApi(fullGameName, TGI_VERSION_MAJOR, TGI_VERSION_MINOR, TGI_VERSION_REVISION, nullptr, 0, analyticalUse);
    }

    inline ITobiiGameIntegrationApi* GetApi(const char* fullGameName, const uint16_t* license, uint32_t licenseSize, bool analyticalUse = false)
    {
        return GetApi(fullGameName, TGI_VERSION_MAJOR, TGI_VERSION_MINOR, TGI_VERSION_REVISION, license, licenseSize, analyticalUse);
    }

    inline ITobiiGameIntegrationApi* GetApiDynamic(const char* fullGameName, const char* dllPath, bool analyticalUse = false)
    {
        return GetApiDynamic(fullGameName, dllPath, TGI_VERSION_MAJOR, TGI_VERSION_MINOR, TGI_VERSION_REVISION, nullptr, 0, analyticalUse);
    }

    inline ITobiiGameIntegrationApi* GetApiDynamic(const char* fullGameName, const char* dllPath, const uint16_t* license, uint32_t licenseSize, bool analyticalUse = false)
    {
        return GetApiDynamic(fullGameName, dllPath, TGI_VERSION_MAJOR, TGI_VERSION_MINOR, TGI_VERSION_REVISION, license, licenseSize, analyticalUse);
    }

    inline ITobiiGameIntegrationApi* GetApiDynamic(const char* fullGameName, bool analyticalUse = false)
    {
        return GetApiDynamic(fullGameName, nullptr, TGI_VERSION_MAJOR, TGI_VERSION_MINOR, TGI_VERSION_REVISION, nullptr, 0, analyticalUse);
    }

    inline ITobiiGameIntegrationApi* GetApiDynamic(const char* fullGameName, const uint16_t* license, uint32_t licenseSize, bool analyticalUse = false)
    {
        return GetApiDynamic(fullGameName, nullptr, TGI_VERSION_MAJOR, TGI_VERSION_MINOR, TGI_VERSION_REVISION, license, licenseSize, analyticalUse);
    }

#pragma region Common types

    struct TGI_TYPE_EXPORT Vector2d
    {
        float X;
        float Y;
    };

    struct TGI_TYPE_EXPORT Vector3d
    {
        float X;
        float Y;
        float Z;
    };

    struct TGI_TYPE_EXPORT Matrix4x4
    {
        float ColumnMajorData[16];
    };

    enum class HMDValidityFlags
    {
        LeftEyeIsValid = 1 << 0,
        RightEyeIsValid = 1 << 1
    };
    inline HMDValidityFlags operator |(HMDValidityFlags a, HMDValidityFlags b) { return static_cast<HMDValidityFlags>(static_cast<uint32_t>(a) | static_cast<uint32_t>(b)); }
    inline HMDValidityFlags& operator |=(HMDValidityFlags &a, HMDValidityFlags b) { return a = a | b; }
    inline HMDValidityFlags operator &(HMDValidityFlags a, HMDValidityFlags b) { return static_cast<HMDValidityFlags>(static_cast<uint32_t>(a) & static_cast<uint32_t>(b)); }
    inline HMDValidityFlags& operator &=(HMDValidityFlags& a, HMDValidityFlags b) { return a = a & b; }

    struct TGI_TYPE_EXPORT EyeInfo
    {
        EyeInfo() :
            GazeOriginMM{ 0.0f, 0.0f, 0.0f },
            GazeDirection{ 0.0f, 0.0f, 0.0f },
            PupilPosition{ 0.0f, 0.0f },
            EyeOpenness(0.0f)
        { }

        Vector3d GazeOriginMM;
        Vector3d GazeDirection;
        Vector2d PupilPosition;
        float EyeOpenness;
    };

    struct TGI_TYPE_EXPORT HMDGaze
    {
        HMDGaze() :
            Timestamp(0xDEADBEEF),
            Validity(HMDValidityFlags::LeftEyeIsValid & HMDValidityFlags::RightEyeIsValid),
            //Counter(0),
            //LedMode(0),
            LeftEyeInfo(),
            RightEyeInfo()
        { }

        int64_t Timestamp;
        HMDValidityFlags Validity;
        //uint32_t Counter;
        //uint32_t LedMode;
        EyeInfo LeftEyeInfo;
        EyeInfo RightEyeInfo;
    };

    struct TGI_TYPE_EXPORT Quaternion
    {
        float X;
        float Y;
        float Z;
        float W;
    };

    struct TGI_TYPE_EXPORT Rectangle
    {
        int32_t Left;
        int32_t Top;
        int32_t Right;
        int32_t Bottom;
    };

    struct TGI_TYPE_EXPORT Dimensions
    {
        int32_t Width;
        int32_t Height;
    };

    enum UnitType
    {
        SignedNormalized = 0,   //  Gaze point, signed normalized, client window bottom, left = (-1, -1), client window top, right = (1, 1)
        Normalized,             //  Gaze point, unsigned normalized, client window bottom, left = (0, 0), client window top, right = (1, 1)
        Mm,                     //  Gaze point, mm, client window bottom, left = (0, 0), client window top, right = (window_width_mm, window_height_mm)
        Pixels,                 //  Gaze point, pixel, client window bottom, left = (0, 0), client window top, right = (window_width_pixels, window_height_pixels)
        NumberOfUnitTypes       //	Use for looping, array allocation etc 
    };


    template <typename T>
    struct TGI_TYPE_EXPORT Range
    {
        T   Min;
        T   Max;

        Range(T min, T max) noexcept : Min(min), Max(max) {}
        Range() {}
    };

    template <typename T>
    struct TGI_TYPE_EXPORT SettingMetadata
    {
        const T           Default;
        const Range<T>    MinMaxRange;

        SettingMetadata(const SettingMetadata<T>& other) :
            Default(other.Default), MinMaxRange(other.MinMaxRange)
        { }

        SettingMetadata(T defaultValue, Range<T> minMaxRange) noexcept :
            Default(defaultValue), MinMaxRange(minMaxRange)
        { }
    };

    template <typename T>
    struct TGI_TYPE_EXPORT Setting
    {
        T Value;
        const SettingMetadata<T> Metadata;

        Setting(T defaultValue, Range<T> minMaxRange) :
            Value(defaultValue), Metadata(defaultValue, minMaxRange)
        { }

        Setting(const Setting<T>& other) :
            Value(other.Value), Metadata(other.Metadata)
        { }

        bool Clamp() 
        {
            bool wasClamped = true;
            Value > Metadata.MinMaxRange.Max ?
                Value = Metadata.MinMaxRange.Max :
                (Value < Metadata.MinMaxRange.Min ? Value = Metadata.MinMaxRange.Min : wasClamped = false);
            return wasClamped;
        }

        operator T& () { return Value; }
        operator const T& () const { return Value; }
        Setting<T>& operator=(const T& other) { Value = other; return *this; }
        Setting<T>& operator=(const Setting<T>& other) { Value = other.Value; return *this; }
        bool operator==(const Setting<T>& other) const { return Value == other.Value; }
    };

#pragma endregion

#pragma region Eye tracking specific types

    struct TGI_TYPE_EXPORT GazePoint
    {
        int64_t TimeStampMicroSeconds;
        float X;
        float Y;

        GazePoint()
            : TimeStampMicroSeconds(0)
            , X(0.0f)
            , Y(0.0f)
        { }
    };

    struct TGI_TYPE_EXPORT Rotation
    {
        float YawDegrees;	// Clockwise rotation about the down vector, the angle increases when turning your head right
        float PitchDegrees;	// Clockwise rotation about the right vector, the angle increases when turning your head up
        float RollDegrees;	// Clockwise rotation about the forward vector, the angle increases when tilting your head to the right

        Rotation()
            : YawDegrees(0.0f)
            , PitchDegrees(0.0f)
            , RollDegrees(0.0f)
        { }
    };

    struct TGI_TYPE_EXPORT Position
    {
        float X;	// Increases when moving your head to the right
        float Y;	// Increases when moving your head up
        float Z;	// Increases when moving away from the tracker

        Position()
            : X(0.0f)
            , Y(0.0f)
            , Z(0.0f)
        { }
    };

    struct TGI_TYPE_EXPORT Transformation
    {
        Rotation Rotation;
        Position Position;

        Transformation()
            : Rotation()
            , Position()
        { }
    };

    struct TGI_TYPE_EXPORT HeadPose : Transformation
    {
        int64_t TimeStampMicroSeconds;

        HeadPose()
            : TimeStampMicroSeconds(0)
        { }
    };

    struct TGI_TYPE_EXPORT AxisSettings
    {
        Setting<float> Limit;
        Setting<float> SensitivityScaling;
        Setting<float> SCurveStrengthNorm;
        Setting<float> SCurveMidPointNorm;
        Setting<float> DeadZoneNorm;

        AxisSettings(Setting<float> limit, Setting<float> sensitivityScaling, Setting<float> sCurveStrengthNorm,
            Setting<float> sCurveMidPointNorm, Setting<float> deadZoneNorm)
            : Limit(limit)
            , SensitivityScaling(sensitivityScaling)
            , SCurveStrengthNorm(sCurveStrengthNorm)
            , SCurveMidPointNorm(sCurveMidPointNorm)
            , DeadZoneNorm(deadZoneNorm)
        { }

        int PlotCurvePoints(Vector2d allocatedPointsBuf[], int maxPoints, bool scalePointsToLimit = false) const;
    };

    enum class Axis { Yaw = 0, Pitch, Roll, X, Y, Z, Count };
    enum class AxisFlags
    {
        None = 0,
        Yaw = 1 << (int)Axis::Yaw,
        Pitch = 1 << (int)Axis::Pitch,
        Roll = 1 << (int)Axis::Roll,
        AllRotation = Yaw | Pitch | Roll,
        X = 1 << (int)Axis::X,
        Y = 1 << (int)Axis::Y,
        Z = 1 << (int)Axis::Z,
        AllPosition = X | Y | Z
    };

    inline AxisFlags  operator |(AxisFlags a, AxisFlags b) { return static_cast<AxisFlags>(static_cast<int>(a) | static_cast<int>(b)); }
    inline AxisFlags& operator |=(AxisFlags& a, AxisFlags b) { return a = a | b; }
    inline AxisFlags  operator &(AxisFlags a, AxisFlags b) { return static_cast<AxisFlags>(static_cast<int>(a)& static_cast<int>(b)); }
    inline AxisFlags& operator &=(AxisFlags& a, AxisFlags b) { return a = a & b; }

    enum class AxisDirection { Negative = 0, Positive, Count };
    enum class AxisDirectionFlags
    {
        None = 0,
        Negative = 1 << (int)AxisDirection::Negative,
        Positive = 1 << (int)AxisDirection::Positive,
        Both = Positive | Negative
    };

    inline AxisDirectionFlags  operator |(AxisDirectionFlags a, AxisDirectionFlags b) { return static_cast<AxisDirectionFlags>(static_cast<int>(a) | static_cast<int>(b)); }
    inline AxisDirectionFlags& operator |=(AxisDirectionFlags& a, AxisDirectionFlags b) { return a = a | b; }
    inline AxisDirectionFlags  operator &(AxisDirectionFlags a, AxisDirectionFlags b) { return static_cast<AxisDirectionFlags>(static_cast<int>(a)& static_cast<int>(b)); }
    inline AxisDirectionFlags& operator &=(AxisDirectionFlags& a, AxisDirectionFlags b) { return a = a & b; }


#pragma endregion

    struct TGI_TYPE_EXPORT HeadTrackingSettings
    {
        Setting<bool>  Enabled{ true, Range<bool>(false, true) };
        Setting<bool>  AutoReset{ true, Range<bool>(false, true) };
        Setting<float> RotationResponsiveness{ 1.0f, Range<float>(0.0f, 1.0f) };
        Setting<bool>  RotationRollEnabled{ false, Range<bool>(false, true) };
        Setting<bool>  PositionEnabled{ false, Range<bool>(false, true) };
        Setting<bool>  RelativeHeadPositionEnabled{ true, Range<bool>(false, true) };
        Setting<bool>  RotateAxisSettingsWithHead{ false, Range<bool>(false, true) };

        AxisSettings YawRightDegrees{   { 180.0, Range<float>(0.0f, 180.0f)},       // Limit
                                        { 2.0f, Range<float>(0.0f, 12.0f)},         // SensitivityScaling
                                        { 0.0f, Range<float>(0.0f, 1.0f)},          // SCurveStrengthNorm
                                        { 0.5f, Range<float>(0.0f, 1.0f)},          // SCurveMidPointNorm
                                        { 0.0f, Range<float>(0.0f, 1.0f)} };        // DeadZoneNorm

        AxisSettings YawLeftDegrees{    { -180.0f, Range<float>(-180.0f, 0.0f)},    // Limit
                                        { 2.0f, Range<float>(0.0f, 12.0f)},         // SensitivityScaling
                                        { 0.0f, Range<float>(0.0f, 1.0f)},          // SCurveStrengthNorm
                                        { 0.5f, Range<float>(0.0f, 1.0f)},          // SCurveMidPointNorm
                                        { 0.0f, Range<float>(0.0f, 1.0f)} };        // DeadZoneNorm

        AxisSettings PitchUpDegrees{    { 90.0f, Range<float>(0.0f, 90.0f)},        // Limit
                                        { 2.0f, Range<float>(0.0f, 12.0f)},         // SensitivityScaling
                                        { 0.0f, Range<float>(0.0f, 1.0f)},          // SCurveStrengthNorm
                                        { 0.5f, Range<float>(0.0f, 1.0f)},          // SCurveMidPointNorm
                                        { 0.0f, Range<float>(0.0f, 1.0f)} };        // DeadZoneNorm

        AxisSettings PitchDownDegrees{  { -90.0f, Range<float>(-90.0f, 0.0f)},      // Limit
                                        { 2.0f, Range<float>(0.0f, 12.0f)},         // SensitivityScaling
                                        { 0.0f, Range<float>(0.0f, 1.0f)},          // SCurveStrengthNorm
                                        { 0.5f, Range<float>(0.0f, 1.0f)},          // SCurveMidPointNorm
                                        { 0.0f, Range<float>(0.0f, 1.0f)} };        // DeadZoneNorm

        AxisSettings RollRightDegrees{  { 90.0f, Range<float>(0.0f, 90.0f)},        // Limit
                                        { 1.0f, Range<float>(0.0f, 12.0f)},         // SensitivityScaling
                                        { 0.0f, Range<float>(0.0f, 1.0f)},          // SCurveStrengthNorm
                                        { 0.5f, Range<float>(0.0f, 1.0f)},          // SCurveMidPointNorm
                                        { 0.0f, Range<float>(0.0f, 1.0f)} };        // DeadZoneNorm

        AxisSettings RollLeftDegrees{   { -90.0f, Range<float>(-90.0f, 0.0f)},      // Limit
                                        { 1.0f, Range<float>(0.0f, 12.0f)},         // SensitivityScaling
                                        { 0.0f, Range<float>(0.0f, 1.0f)},          // SCurveStrengthNorm
                                        { 0.5f, Range<float>(0.0f, 1.0f)},          // SCurveMidPointNorm
                                        { 0.0f, Range<float>(0.0f, 1.0f)} };        // DeadZoneNorm

        AxisSettings XRightMm{          { 300.0f, Range<float>(0.0f, 500.0f)},      // Limit
                                        { 1.0f, Range<float>(0.0f, 12.0f)},         // SensitivityScaling
                                        { 0.0f, Range<float>(0.0f, 1.0f)},          // SCurveStrengthNorm
                                        { 0.5f, Range<float>(0.0f, 1.0f)},          // SCurveMidPointNorm
                                        { 0.0f, Range<float>(0.0f, 1.0f)} };        // DeadZoneNorm

        AxisSettings XLeftMm{           { -300.0f, Range<float>(-500.0f, 0.0f)},    // Limit
                                        { 1.0f, Range<float>(0.0f, 12.0f)},         // SensitivityScaling
                                        { 0.0f, Range<float>(0.0f, 1.0f)},          // SCurveStrengthNorm
                                        { 0.5f, Range<float>(0.0f, 1.0f)},          // SCurveMidPointNorm
                                        { 0.0f, Range<float>(0.0f, 1.0f)} };        // DeadZoneNorm

        AxisSettings YUpMm{             { 300.0f, Range<float>(0.0f, 500.0f)},      // Limit
                                        { 1.0f, Range<float>(0.0f, 12.0f)},         // SensitivityScaling
                                        { 0.0f, Range<float>(0.0f, 1.0f)},          // SCurveStrengthNorm
                                        { 0.5f, Range<float>(0.0f, 1.0f)},          // SCurveMidPointNorm
                                        { 0.0f, Range<float>(0.0f, 1.0f)} };        // DeadZoneNorm

        AxisSettings YDownMm{           { -300.0f, Range<float>(-500.0f, 0.0f)},    // Limit
                                        { 1.0f, Range<float>(0.0f, 12.0f)},         // SensitivityScaling
                                        { 0.0f, Range<float>(0.0f, 1.0f)},          // SCurveStrengthNorm
                                        { 0.5f, Range<float>(0.0f, 1.0f)},          // SCurveMidPointNorm
                                        { 0.0f, Range<float>(0.0f, 1.0f)} };        // DeadZoneNorm

        AxisSettings ZBackMm{           { 300.0f, Range<float>(0.0f, 500.0f)},      // Limit
                                        { 1.0f, Range<float>(0.0f, 12.0f)},         // SensitivityScaling
                                        { 0.0f, Range<float>(0.0f, 1.0f)},          // SCurveStrengthNorm
                                        { 0.5f, Range<float>(0.0f, 1.0f)},          // SCurveMidPointNorm
                                        { 0.0f, Range<float>(0.0f, 1.0f)} };        // DeadZoneNorm

        AxisSettings ZForwardMm{        { -300.0f, Range<float>(-500.0f, 0.0f)},    // Limit
                                        { 1.0f, Range<float>(0.0f, 12.0f)},         // SensitivityScaling
                                        { 0.0f, Range<float>(0.0f, 1.0f)},          // SCurveStrengthNorm
                                        { 0.5f, Range<float>(0.0f, 1.0f)},          // SCurveMidPointNorm
                                        { 0.0f, Range<float>(0.0f, 1.0f)} };        // DeadZoneNorm

    };

    namespace HeadTrackingHelpFunctions
    {
        // Description:
        //      Sets the Value of a Setting<float> member in the AxisSettings of multiple specified head tracking axes.
        //      See example usage of this function in e.g. ExtendedViewSettings::SetHeadAllRotationAxisSettingsSensitivity(float sensitivity)
        //
        // Parameters:
        //  headTrackingSettings
        //      head tracking settings to be updated
        //  axes 
        //      A bit field specifying which axes to choose to operate on
        //  axisDirections
        //      A bit field further specifying which direction(s) of the specified axes to operate on
        //  pAxisSettingsMember
        //      A pointer-to-member specifying which Setting<float> member of AxisSettings of the specified axes to set
        //      Valid usage examples: &AxisSettings::Limit, &AxisSettings::SensitivityScaling, &AxisSettings::SCurveStrengthNorm, &AxisSettings::SCurveMidPointNorm, &AxisSettings::DeadZoneNorm
        //  value
        //      Either, if valueIsNormalizedOnRangeMagnitude is false:
        //          The float value to assign to the Value of the specified AxisSettings member
        //      Or, if valueIsNormalizedOnRangeMagnitude is true:
        //          A normalized float (0.0f to 1.0f), used to interpolate over the specified AxisSettings member's MinMaxRange.
        //          For positive axes, a value of 0.0f maps to the Min of the range, and 1.0f maps to the Max of the range.
        //          For negative axes, a value of 0.0f maps to the Max of the range, and 1.0f maps to the Min of the range. (i.e. value maps to the magnitude of the range)
        //  valueIsNormalizedOnRangeMagnitude
        //      A bool used to specify whether or not value should be interpreted as normalized on the magnitude of the range as described above.
        void TGI_TYPE_EXPORT SetHeadMultiAxisSettingsValues(HeadTrackingSettings& headTrackingSettings, AxisFlags axes, AxisDirectionFlags axisDirections,
            Setting<float> AxisSettings::* pAxisSettingsMember, float value, bool valueIsNormalizedOnRangeMagnitude);

        // Description:
        //      Sets the SensitivityScaling Value of the AxisSettings of all positive and negative head tracking rotation-axes.
        //
        // Parameters:
        //  headTrackingSettings
        //      head tracking settings to be updated        
        //  sensitivityScaling
        //      Acts as a multiplier on the head tracking rotation input to produce the ExtendedView rotation output
        inline void SetHeadAllRotationAxisSettingsSensitivity(HeadTrackingSettings& headTrackingSettings, float sensitivityScaling)
        {
            SetHeadMultiAxisSettingsValues(headTrackingSettings, AxisFlags::AllRotation, AxisDirectionFlags::Both, &AxisSettings::SensitivityScaling, sensitivityScaling, false);
        };

        // Description:
        //      Sets the Limit Value of the AxisSettings of all positive and negative head tracking rotation-axes.
        //
        // Parameters:
        //  headTrackingSettings
        //      head tracking settings to be updated
        //  limitNormalizedOnRange
        //      a normalized float (0.0f to 1.0f) specifying the magnitude of the rotation angle-limit normalized on the MinMaxRange for each AxisSettings
        inline void SetHeadAllRotationAxisSettingsLimitNormalized(HeadTrackingSettings& headTrackingSettings, float limitNormalizedOnRange)
        {
            SetHeadMultiAxisSettingsValues(headTrackingSettings, AxisFlags::AllRotation, AxisDirectionFlags::Both, &AxisSettings::Limit, limitNormalizedOnRange, true);
        };

        // Description:
        //      Sets the SensitivityScaling Value of the AxisSettings of all positive and negative head tracking position-axes.
        //
        // Parameters:
        //  headTrackingSettings
        //      head tracking settings to be updated
        //  sensitivityScaling
        //      Acts as a multiplier on the head tracking position input to produce the ExtendedView position output
        inline void SetHeadAllPositionAxisSettingsSensitivity(HeadTrackingSettings& headTrackingSettings, float sensitivityScaling)
        {
            SetHeadMultiAxisSettingsValues(headTrackingSettings, AxisFlags::AllPosition, AxisDirectionFlags::Both, &AxisSettings::SensitivityScaling, sensitivityScaling, false);
        };

        // Description:
        //      Sets the Limit Value of the AxisSettings of all positive and negative head tracking position-axes.
        //
        // Parameters:
        //  headTrackingSettings
        //      head tracking settings to be updated
        //  limitNormalizedOnRange
        //      a normalized float (0.0f to 1.0f) specifying the magnitude of the position-limit normalized on the MinMaxRange for each AxisSettings
        inline void SetHeadAllPositionAxisSettingsLimitNormalized(HeadTrackingSettings& headTrackingSettings, float limitNormalizedOnRange)
        {
            SetHeadMultiAxisSettingsValues(headTrackingSettings, AxisFlags::AllPosition, AxisDirectionFlags::Both, &AxisSettings::Limit, limitNormalizedOnRange, true);
        };

        // Description:
        //  Increase to make head tracking more stable in the center.
        //  Operates on the AxisSettings members controlling the head tracking dead zone and S-curve of all rotational axes.
        //
        // Parameters:
        //  headTrackingSettings
        //      head tracking settings to be updated
        //  centerStabilization
        //      Range 0.0f to 1.0f
        void TGI_TYPE_EXPORT SetCenterStabilization(HeadTrackingSettings& headTrackingSettings, float centerStabilization);
    };

    struct TGI_TYPE_EXPORT CameraBoostSettings
    {
        Setting<bool>   Enabled { true, Range<bool>(false, true) };
        Setting<float>  GazeDeadZone { 0.5f, Range<float>(0.0f, 0.75f) };
        Setting<float>  Boost { 1.5f, Range<float>(0.0f, 2.0f) };
    };

    struct TGI_TYPE_EXPORT GazeHeadMixSettings
    {
        Setting<bool>   Enabled { false, Range<bool>(false, true) };
        Setting<float>  GazeResponsiveness { 0.5f, Range<float>(0.0f, 1.0f) };
        Setting<float>  GazeYawLimitDegrees { 0.0f, Range<float>(0.0f, 180.0f) };
        Setting<float>  GazePitchUpLimitDegrees { 0.0f, Range<float>(0.0f, 90.0f) };
        Setting<float>  GazePitchDownLimitDegrees { 0.0f, Range<float>(-90.0f, 0.0f) };
    };

    namespace GazeHeadMixHelpFunctions
    {
        // Description:
        //      Sets the ratio between eye- and head tracking.
        //      This will change the distribution of the maximum gaze angles and the maximum head tracking limits in gazeHeadMixSettings and headTrackingSettings
        //
        // Parameters:
        //  gazeHeadMixSettings
        //      gaze settings to be updated
        //  headTrackingSettings
        //      head tracking settings to be updated
        //  eyeHeadTrackingRatio
        //      Range 0.0f to 1.0f
        //      0.0f = only eye tracking, 1.0f = only head tracking

        void TGI_TYPE_EXPORT SetEyeHeadTrackingRatio(GazeHeadMixSettings& gazeHeadMixSettings, HeadTrackingSettings& headTrackingSettings, float eyeHeadTrackingRatio);

        // Description:
        //      Sets the maximum yaw angle extended view can produce in degrees when using GazeHeadMix,
        // 
        // Parameters:
        //  gazeHeadMixSettings
        //      gaze settings to be updated
        //  headTrackingSettings
        //      head tracking settings to be updated
        //  positiveYawLimitDegrees
        //      This value limits the maximum output of rotating your head left or right + looking left or right.
        //      This value is mirrored for left and right rotations and a value of 0 would effectively turn the feature off.
        //      Extended view yaw angle will be capped at this value.
        void TGI_TYPE_EXPORT SetCameraMaxAngleYaw(GazeHeadMixSettings& gazeHeadMixSettings, HeadTrackingSettings& headTrackingSettings, float positiveYawLimitDegrees);

        // Description:
        //      Sets the maximum pitch up angle extended view can produce in degrees when using GazeHeadMix,
        // 
        // Parameters:
        //  gazeHeadMixSettings
        //      gaze settings to be updated
        //  headTrackingSettings
        //      head tracking settings to be updated
        //  positivePitchUpLimitDegrees
        //      This value limits the maximum output of rotating your head upwards + looking up.
        //      Extended view pitch angle will be capped at this value.
        void TGI_TYPE_EXPORT SetCameraMaxAnglePitchUp(GazeHeadMixSettings& gazeHeadMixSettings, HeadTrackingSettings& headTrackingSettings, float positivePitchUpLimitDegrees);

        // Description:
        //  Sets the maximum pitch down angle extended view can produce in degrees when using GazeHeadMix,
        // 
        // Parameters:
        //  gazeHeadMixSettings
        //      gaze settings to be updated
        //  headTrackingSettings
        //      head tracking settings to be updated
        //  negativePitchDownLimitDegrees
        //      This value limits the maximum output of rotating your head downwards + looking down.
        //      Extended view pitch angle will be capped at this value.
        void TGI_TYPE_EXPORT SetCameraMaxAnglePitchDown(GazeHeadMixSettings& gazeHeadMixSettings, HeadTrackingSettings& headTrackingSettings, float negativePitchDownLimitDegrees);
    };

    struct TGI_TYPE_EXPORT ExtendedViewSettings
    {
        HeadTrackingSettings    HeadTracking;
        CameraBoostSettings     CameraBoost;
        GazeHeadMixSettings     GazeHeadMix;
    };

    enum class TrackerType
    {
        None = 0,
        PC,
        HeadMountedDisplay
    };

    enum class StreamType
    {
        Presence = 0,
        Head = 1,
        GazeOS = 2,
        Gaze = 3,
        Foveation = 4,
        EyeInfo = 5,
        HMD = 6,
        UnfilteredGaze = 7,
        Count = 8
    };

    enum class StreamFlags
    {
        None = 0,
        Presence = 1 << (int)StreamType::Presence,
        Head = 1 << (int)StreamType::Head,
        GazeOS = 1 << (int)StreamType::GazeOS,
        Gaze = 1 << (int)StreamType::Gaze,
        Foveation = 1 << (int)StreamType::Foveation,
        EyeInfo = 1 << (int)StreamType::EyeInfo,
        HMD = 1 << (int)StreamType::HMD,
        UnfilteredGaze = 1 << (int)StreamType::UnfilteredGaze
    };

    inline StreamFlags operator |(StreamFlags a, StreamFlags b) { return static_cast<StreamFlags>(static_cast<int>(a) | static_cast<int>(b)); }
    inline StreamFlags& operator |=(StreamFlags &a, StreamFlags b) { return a = a | b; }
    inline StreamFlags operator &(StreamFlags a, StreamFlags b) { return static_cast<StreamFlags>(static_cast<int>(a) & static_cast<int>(b)); }
    inline StreamFlags& operator &=(StreamFlags& a, StreamFlags b) { return a = a & b; }

    struct TGI_TYPE_EXPORT AimAtGazeFilterSettings
    {
        AimAtGazeFilterSettings() :
            IsEnabled(false),
            DistanceThreshold(0.1f)
        {  }

        bool IsEnabled;
        float DistanceThreshold;
    };

    struct TGI_TYPE_EXPORT ResponsiveFilterSettings
    {
        ResponsiveFilterSettings() :
            IsEnabled(false),
            Responsiveness(0.5f),
            StickinessDistance(0.1f)
        {  }

        bool IsEnabled;
        float Responsiveness;
        float StickinessDistance;
    };



    struct TGI_TYPE_EXPORT TrackerInfo
    {
        TrackerInfo() :
            Type(TrackerType::None),
            Capabilities(StreamFlags::None),
            DisplayRectInOSCoordinates({ 0, 0, 0, 0}),
            DisplaySizeMm({ 0, 0 }),
            Url(nullptr),
            FriendlyName(nullptr),
            MonitorNameInOS(nullptr),
            ModelName(nullptr),
            Generation(nullptr),
            SerialNumber(nullptr),
            FirmwareVersion(nullptr),
            IsAttached(false)
        { }

        TrackerType Type;
        StreamFlags Capabilities;
        Rectangle DisplayRectInOSCoordinates;
        Dimensions DisplaySizeMm;
        const char* Url;
        const char* FriendlyName;
        const char* MonitorNameInOS;
        const char* ModelName;
        const char* Generation;
        const char* SerialNumber;
        const char* FirmwareVersion;
        bool IsAttached;
    };

    enum Literal
    {
        IsEnabled,
        IsDisabled,
        ChangedToEnabled,
        ChangedToDisabled,
        GameStarted,
        GameStopped,
        Separator,
        AutoPause,
        CharacterAwareness,
        EnvironmentalAwareness,
        ObjectAwareness,
        BungeeZoom,
        ZoomAtGaze,
        CleanUI,
        DynamicDepthOfField,
        DynamicLightAdaptation,
        FlashlightControl,
        PeripheralEffects,
        AutoTurn,
        ExtendedView,
        FreeView,
        AimAtGaze,
        CoverAtGaze,
        MarkAtGaze,
        InteractAtGaze,
        HackAtGaze,
        ThrowAtGaze,
        TeleportToGaze,
        JumpAtGaze,
        FireAtGaze,
        FoveatedRendering,
        WheelMenu,
        CursorWarp,
        CenterAtGaze,
        DisplayCount,
        EyetrackerInfo,
        DisplayWhereTheGameRuns,
        NumberOfLiterals
    };

    struct Feature
    {
        int Id;
        const char* Name;
        bool Enabled;
    };

    struct TGI_TYPE_EXPORT IStatistics
    {
        virtual void SetFeatureList(const Feature* gameFeatures, int numberOfFeatures) = 0;
        virtual const char* GetLiteral(Literal literal) const = 0;
        virtual void SendFeatureEnabled(int featureId) = 0;
        virtual void SendFeatureDisabled(int featureId) = 0;
        virtual void SendFeaturesState() = 0;
        virtual void StopAllLogging() = 0;
        virtual void ResumeAllLogging() = 0;
    };

    struct TGI_TYPE_EXPORT ITrackerController
    {
        virtual bool GetTrackerInfo(TrackerInfo& trackerInfo) = 0;
        virtual bool GetTrackerInfo(const char* url, TrackerInfo& trackerInfo) = 0;
        virtual void UpdateTrackerInfos() = 0;
        virtual bool GetTrackerInfos(const TrackerInfo*& trackerInfos, int& numberOfTrackerInfos) = 0;
        virtual bool TrackHMD() = 0;
        virtual bool TrackRectangle(const Rectangle& rectangle) = 0;
        virtual bool TrackWindow(void* windowHandle) = 0;
        virtual void StopTracking() = 0;
        virtual bool IsConnected() const = 0;
        virtual bool IsEnabled() const = 0;
        virtual bool IsStreamSupported(const StreamFlags& stream) const = 0;
    };

    struct TGI_TYPE_EXPORT IStreamsProvider
    {
        virtual int GetHeadPoses(const HeadPose*& headPoses) = 0;
        virtual bool GetLatestHeadPose(HeadPose& headPose) = 0;

        virtual int GetGazePoints(const GazePoint*& gazePoints) = 0;
        virtual bool GetLatestGazePoint(GazePoint& gazePoint) = 0;

        virtual int GetHMDGaze(const HMDGaze*& hmdGaze) = 0;
        virtual bool GetLatestHMDGaze(HMDGaze& latestHMDGaze) = 0;

        virtual bool IsPresent() = 0;

        virtual void SetAutoUnsubscribe(StreamType stream, float timeout) = 0;
        virtual void UnsetAutoUnsubscribe(StreamType stream) = 0;

        virtual void ConvertGazePoint(const GazePoint& fromGazePoint, GazePoint& toGazePoint, UnitType fromUnit, UnitType toUnit) = 0;
    };

    struct TGI_TYPE_EXPORT IExtendedView
    {
        virtual Transformation GetTransformation() = 0;
        virtual bool UpdateSettings(const ExtendedViewSettings& settings) = 0;
        virtual void ResetDefaultHeadPose() = 0;
        virtual void Pause(bool reCenter, float transitionDuration = 0.2f) = 0;
        virtual void UnPause(float transitionDuration = 0.2f) = 0;
        virtual bool IsPaused() = 0;
        virtual void GetSettings(ExtendedViewSettings& settings) const = 0;
    };

    struct TGI_TYPE_EXPORT IFeatures
    {
        virtual IExtendedView* GetExtendedView() = 0;
    };

    struct TGI_TYPE_EXPORT IFilters
    {
        virtual const ResponsiveFilterSettings& GetResponsiveFilterSettings() const = 0;
        virtual void SetResponsiveFilterSettings(ResponsiveFilterSettings settings) = 0;
        virtual const AimAtGazeFilterSettings& GetAimAtGazeFilterSettings() const = 0;
        virtual void SetAimAtGazeFilterSettings(AimAtGazeFilterSettings settings) = 0;

        virtual void GetResponsiveFilterGazePoint(GazePoint& gazePoint) const  = 0;
        virtual void GetAimAtGazeFilterGazePoint(GazePoint& gazePoint, float &gazePointStability) const = 0;
    };

    struct TGI_TYPE_EXPORT ITobiiGameIntegrationApi
    {
        virtual ITrackerController* GetTrackerController() = 0;
        virtual IStreamsProvider* GetStreamsProvider() = 0;
        virtual IFeatures* GetFeatures() = 0;
        virtual IStatistics* GetStatistics() = 0;
        virtual IFilters* GetFilters() = 0;

        virtual bool IsInitialized() = 0;
        virtual void Update() = 0;
        virtual void Shutdown() = 0;
    };
}
